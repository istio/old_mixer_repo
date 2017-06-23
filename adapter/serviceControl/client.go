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

package serviceControl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	servicecontrol "google.golang.org/api/servicecontrol/v1"

	"istio.io/mixer/pkg/adapter"
)

func createAPIClient(logger adapter.Logger, clientCredentialPath string) (*servicecontrol.Service, error) {
	logger.Infof("Creating service control client...\n")
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{
		Transport: http.DefaultTransport})

	bytes, err := ioutil.ReadFile(clientCredentialPath + "secret.json")
	if err != nil {
		return nil, err
	}

	o, err := google.ConfigFromJSON(bytes, servicecontrol.CloudPlatformScope, servicecontrol.ServicecontrolScope)
	if err != nil {
		return nil, err
	}
	logger.Infof("Created oauth config %v\n", o)

	// The following code is uncommented for authorizing the client for using oauth2 the first time.
	//
	//	authorize(ctx, *o)

	// t, err := o.Exchange(ctx, "<fill in the code>")

//		if err != nil {
//			return nil, err
//		}
//		showToken(t)

	t, err := tokenFromFile(clientCredentialPath + "token.json")

	if err != nil {
		return nil, err
	}
	o.TokenSource(ctx, t)

	if err != nil {
		return nil, err
	}

	httpClient := o.Client(ctx, t)
	s, err := servicecontrol.New(httpClient)
	logger.Infof("Created service control client")
	return s, err
}

func authorize(ctx context.Context, config oauth2.Config) {
	authURL := config.AuthCodeURL("")

	showURL(authURL)

	return
}

func showURL(url string) {
	fmt.Printf("Authorization URL and copy the code: \n%s\n\n", url)
}

func obtainCode() (string, error) {
	var code string
	_, err := fmt.Scanln(&code)
	return code, err
}

func showToken(token *oauth2.Token) error {
	jt, err := json.Marshal(token)
	if err != nil {
		return err
	}
	fmt.Printf("Obtained token:\n%s\n\n", string(jt))
	return nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()

	if err != nil {
		return nil, err
	}

	jt, err := ioutil.ReadAll(f)

	if err != nil {
		return nil, err
	}

	t := new(oauth2.Token)
	err = json.Unmarshal(jt, &t)
	return t, err
}
