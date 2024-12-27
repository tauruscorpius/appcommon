package LookupArgs

import (
	"errors"
	"flag"
	"github.com/tauruscorpius/appcommon/Log"
	"github.com/tauruscorpius/appcommon/Lookup/LookupConsts"
	"net"
	"os"
	"path"
	"strings"
	"sync"
)

type LookupAppArgs struct {
	ServerHost  string
	AppName     string
	Identifier  string
	BindAddrAny bool
	NodeLookup  []string
	NodeType    LookupConsts.ServiceNodeType
}

var (
	once    sync.Once
	appArgs *LookupAppArgs
)

func hostCheck(in string) error {
	host, port, err := net.SplitHostPort(in)
	if err != nil {
		return err
	}
	if host == "0.0.0.0" || host == "" {
		return errors.New("expose address / any addr disabled")
	}
	if port == "" {
		return errors.New("expose address / miss port")
	}
	return nil
}

func GetLookupAppArgs() *LookupAppArgs {
	once.Do(func() {
		appArgs = &LookupAppArgs{}
	})
	return appArgs
}

func (t *LookupAppArgs) ProcessAppArgs() bool {
	var lookUpHost string
	var addressAny bool
	flag.StringVar(&t.ServerHost, "host", "", "local bind host")
	flag.StringVar(&lookUpHost, "Lookup", "", "Lookup host")
	flag.BoolVar(&addressAny, "any", false, "bind address any")
	flag.Parse()

	// host
	if err := hostCheck(t.ServerHost); err != nil {
		Log.Errorf("error host, error : %v\n", err)
		return false
	}
	t.AppName = path.Base(os.Args[0])
	t.Identifier = strings.ReplaceAll(t.ServerHost, ".", "")
	t.Identifier = strings.ReplaceAll(t.Identifier, ":", "_")

	// node Lookup
	if lookUpHost == "" {
		envLookup := "NODE_LOOKUP"
		lookUpHost = os.Getenv(envLookup)
		if lookUpHost == "" {
			Log.Critical("Neither env NODE_LOOKUP nor arg Lookup exists\n")
			return false
		}
		Log.Criticalf("Using env %s value [%s]\n", envLookup, lookUpHost)
	}
	slice := strings.Split(lookUpHost, ",")
	for _, v := range slice {
		if err := hostCheck(v); err != nil {
			Log.Errorf("error Lookup %s, error : %v\n", v, err)
			return false
		}
		t.NodeLookup = append(t.NodeLookup, v)
	}
	if len(t.NodeLookup) == 0 {
		Log.Errorf("error nil Lookup nodes\n")
		return false
	}
	t.BindAddrAny = addressAny
	return true
}

func (t *LookupAppArgs) SetServiceNodeType(nodeType LookupConsts.ServiceNodeType) {
	t.NodeType = nodeType
}
