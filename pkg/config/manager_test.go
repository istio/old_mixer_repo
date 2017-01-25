package config

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"istio.io/mixer/pkg/adapter"

	"sync"
	"time"

	"istio.io/mixer/pkg/expr"
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
	called int
	rt     *Runtime
	sync.Mutex
}

func (f *fakelistener) ConfigChange(cfg *Runtime) {
	f.Lock()
	f.rt = cfg
	f.called++
	f.Unlock()
}
func (f *fakelistener) Called() int {
	f.Lock()
	called := f.called
	f.Unlock()
	return called
}

func TestConfigManager(t *testing.T) {
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
		loopDelay := time.Millisecond * 50
		vf := &fakeVFinder{v: mt.v}
		ma := &managerArgs{
			aspectFinder:  vf,
			builderFinder: vf,
			eval:          evaluator,
			loopDelay:     loopDelay,
		}
		if mt.gc != "" {
			tmpfile, _ := ioutil.TempFile("", mt.gc)
			ma.globalConfig = tmpfile.Name()
			defer func() { _ = os.Remove(ma.globalConfig) }()
			_, _ = tmpfile.Write([]byte(mt.gcContent))
			_ = tmpfile.Close()
		}

		if mt.sc != "" {
			tmpfile, _ := ioutil.TempFile("", mt.sc)
			ma.serviceConfig = tmpfile.Name()
			defer func() { _ = os.Remove(ma.serviceConfig) }()
			_, _ = tmpfile.Write([]byte(mt.scContent))
			_ = tmpfile.Close()
		}
		testConfigManager(t, newmanager(ma), mt, idx, loopDelay)
	}
}

func newmanager(args *managerArgs) *Manager {
	return NewManager(args.eval, args.aspectFinder, args.builderFinder, args.globalConfig, args.serviceConfig, args.loopDelay)
}

type managerArgs struct {
	eval          expr.Evaluator
	aspectFinder  ValidatorFinder
	builderFinder ValidatorFinder
	loopDelay     time.Duration
	globalConfig  string
	serviceConfig string
}

func testConfigManager(t *testing.T, mgr *Manager, mt mtest, idx int, loopDelay time.Duration) {
	fl := &fakelistener{}
	mgr.Register(fl)

	mgr.Start()
	defer mgr.Close()

	le := mgr.LastError()

	if mt.errStr != "" && le == nil {
		t.Errorf("[%d] Expected an error %s Got nothing", idx, mt.errStr)
		return
	}

	if mt.errStr == "" && le != nil {
		t.Errorf("[%d] Unexpected an error %s", idx, le)
		return
	}

	if mt.errStr == "" && fl.rt == nil {
		t.Errorf("[%d] Config listener was not notified", idx)
	}

	if mt.errStr == "" && le == nil {
		called := fl.Called()
		if le == nil && called != 1 {
			t.Errorf("called Got: %d, want: 1", called)
		}
		// give mgr time to go thru the start Loop() go routine
		// fetchAndNotify should be indirectly called multiple times.
		time.Sleep(loopDelay * 2)
		// check again. should not change, no new data is available
		called = fl.Called()
		if le == nil && called != 1 {
			t.Errorf("called Got: %d, want: 1", called)
		}
		return
	}

	if !strings.Contains(le.Error(), mt.errStr) {
		t.Errorf("[%d] Unexpected error. Expected %s\nGot: %s\n", idx, mt.errStr, le)
		return
	}
}
