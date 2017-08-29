// Copyright 2017 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
)

type perKindValidateFunc func(br *BackEndResource) error

type perKindValidator struct {
	pbSpec   proto.Message
	validate perKindValidateFunc
}

func (pv *perKindValidator) validateAndConvert(key Key, br *BackEndResource, res *Resource) error {
	if pv.validate != nil {
		if err := pv.validate(br); err != nil {
			return err
		}
	}
	res.Spec = proto.Clone(pv.pbSpec)
	return convert(key, br.Spec, res.Spec)
}

func validateRule(br *BackEndResource) error {
	_, matchExists := br.Spec[matchField]
	_, selectorExists := br.Spec[selectorField]
	if !matchExists && selectorExists {
		return errors.New("field 'selector' is deprecated, use 'match' instead")
	}
	if selectorExists {
		glog.Warningf("Deprecated field 'selector' used in %s. Use 'match' instead.", br.Metadata.Name)
	}
	return nil
}

type validateRequest struct {
	Operation ChangeType
	Resources []*BackEndResource
}

type validateResponse struct {
	Allowed bool
	Details []string
}

// ValidatorServer is an https server which triggers the validation of configs
// through external admission webhook.
type ValidatorServer struct {
	targetNS          map[string]bool
	externalValidator Validator
	perKindValidators map[string]*perKindValidator
}

// NewValidatorServer creates a new ValidatorServer.
func NewValidatorServer(
	targetNamespaces []string,
	kinds map[string]proto.Message,
	validator Validator) *ValidatorServer {
	vs := make(map[string]*perKindValidator, len(kinds))
	for k, pb := range kinds {
		var validateFunc perKindValidateFunc
		if k == ruleKind {
			validateFunc = validateRule
		}
		vs[k] = &perKindValidator{pb, validateFunc}
	}
	var targetNS map[string]bool
	if len(targetNamespaces) > 0 {
		targetNS = map[string]bool{}
		for _, ns := range targetNamespaces {
			targetNS[ns] = true
		}
	}
	return &ValidatorServer{
		targetNS:          targetNS,
		externalValidator: validator,
		perKindValidators: vs,
	}
}

func errorToResponse(err error) validateResponse {
	if err == nil {
		return validateResponse{Allowed: true}
	}
	resp := validateResponse{Allowed: false}
	if merr, ok := err.(*multierror.Error); !ok {
		resp.Details = []string{err.Error()}
	} else {
		resp.Details = make([]string, len(merr.Errors))
		for i, e := range merr.Errors {
			resp.Details[i] = e.Error()
		}
	}
	return resp
}

func (v *ValidatorServer) parseRequest(r *http.Request) (*validateRequest, error) {
	// verify the content type is accurate
	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		return nil, fmt.Errorf("contentType=%s, expect application/json", contentType)
	}
	req := &validateRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return req, nil
}

func (v *ValidatorServer) validate(req *validateRequest) error {
	evs := make([]*Event, 0, len(req.Resources))
	keys := make(map[Key]bool, len(req.Resources))
	var merr error
	for _, resource := range req.Resources {
		if v.targetNS != nil && !v.targetNS[resource.Metadata.Namespace] {
			// Ignore resources which are not in the target namespaces.
			continue
		}
		pkv, ok := v.perKindValidators[resource.Kind]
		if !ok {
			// Pass unrecognized kinds -- they should be validated by somewhere else.
			glog.V(3).Infof("unrecognized kind %s is requested to validate", resource.Kind)
			continue
		}
		ev := &Event{
			Type: req.Operation,
			Key:  resource.Key(),
		}
		if keys[ev.Key] {
			if req.Operation == Update {
				merr = multierror.Append(merr, fmt.Errorf("resource %s appears multiple times", ev.Key))
			}
			continue
		}
		keys[ev.Key] = true
		if req.Operation == Update {
			ev.Value = &Resource{Metadata: resource.Metadata}
			if err := pkv.validateAndConvert(ev.Key, resource, ev.Value); err != nil {
				merr = multierror.Append(merr, err)
				continue
			}
		}
		evs = append(evs, ev)
	}
	if merr != nil {
		return merr
	}
	if len(evs) == 0 || v.externalValidator == nil {
		return nil
	}
	return v.externalValidator.Validate(evs)
}

func (v *ValidatorServer) doValidate(r *http.Request) []byte {
	if r.ContentLength == 0 {
		return []byte(`{"allowed": true, "details":["nothing to be validated"]}`)
	}
	req, err := v.parseRequest(r)
	if err == nil {
		glog.V(7).Infof("received: %+v", req)
		err = v.validate(req)
	}
	glog.V(7).Infof("response: %v", err)
	resp, marshalErr := json.Marshal(errorToResponse(err))
	if marshalErr != nil {
		return []byte(fmt.Sprintf(`{"allowed": false, "details": ["%s"]}`, marshalErr.Error()))
	}
	return resp
}

func (v *ValidatorServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write(v.doValidate(r)); err != nil {
		glog.Error(err)
	}
}
