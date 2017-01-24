package config

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"istio.io/mixer/pkg/adapter"

	"time"
)

type mtest struct {
	gcContent string
	gc        string
	scContent string
	sc        string
	v         map[string]adapter.ConfigValidator
	errStr    string
}

type fakelistener struct {
	rt *Runtime
}

func (f *fakelistener) ConfigChange(cfg *Runtime) {
	f.rt = cfg
}

func TestConfigManagerError(t *testing.T) {
	evaluator := newFakeExpr()
	mlist := []mtest{
		{"", "", "", "", nil, "no such file or directory"},
		{sGlobalConfig, "globalconfig", "", "", nil, "no such file or directory"},
		{sGlobalConfig, "globalconfig", sSvcConfig, "serviceconfig", nil, "failed validation"},
		{sGlobalConfigValid, "globalconfig", sSvcConfig2, "serviceconfig", map[string]adapter.ConfigValidator{
			"istio/denychecker": &lc{},
			"metrics":           &lc{},
			"listchecker":       &lc{},
		}, ""},
	}
	for idx, mt := range mlist {
		vf := &fakeVFinder{v: mt.v}
		ma := &ManagerArgs{
			AspectF:  vf,
			BuilderF: vf,
			Eval:     evaluator,
		}
		if mt.gc != "" {
			tmpfile, _ := ioutil.TempFile("", mt.gc)
			ma.GlobalConfig = tmpfile.Name()
			defer func() { _ = os.Remove(ma.GlobalConfig) }()
			_, _ = tmpfile.Write([]byte(mt.gcContent))
			_ = tmpfile.Close()
		}

		if mt.sc != "" {
			tmpfile, _ := ioutil.TempFile("", mt.sc)
			ma.ServiceConfig = tmpfile.Name()
			defer func() { _ = os.Remove(ma.ServiceConfig) }()
			_, _ = tmpfile.Write([]byte(mt.scContent))
			_ = tmpfile.Close()
		}

		mgr := NewManager(ma)
		fl := &fakelistener{}
		mgr.Register(fl)
		mgr.loopDelay = time.Millisecond * 300

		_ = mgr.fetchAndNotify()
		mgr.Start()

		if mt.errStr != "" && mgr.lastError == nil {
			t.Errorf("[%d] Expected an error %s Got nothing", idx, mt.errStr)
			continue
		}

		if mt.errStr == "" && mgr.lastError != nil {
			t.Errorf("[%d] Unexpected an error %s", idx, mgr.lastError)
			continue
		}

		if mt.errStr == "" && fl.rt == nil {
			t.Errorf("[%d] Config listener was not notified", idx)
		}

		if mt.errStr == "" && mgr.lastError == nil {
			continue
		}

		if !strings.Contains(mgr.lastError.Error(), mt.errStr) {
			t.Errorf("[%d] Unexpected error. Expected %s\nGot: %s\n", idx, mt.errStr, mgr.lastError)
		}

		time.Sleep(time.Second * 1)
		mgr.Close()

	}
}
