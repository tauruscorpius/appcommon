package AppCommon

import (
	"github.com/tauruscorpius/appcommon/ApiService"
	"github.com/tauruscorpius/appcommon/Log"
	"github.com/tauruscorpius/appcommon/Lookup"
	"github.com/tauruscorpius/appcommon/Lookup/LookupArgs"
	"github.com/tauruscorpius/appcommon/Lookup/LookupConsts"
	"github.com/tauruscorpius/appcommon/Lookup/LookupDS"
	"github.com/tauruscorpius/appcommon/Lookup/LookupHook"
	"github.com/tauruscorpius/appcommon/Utility/Stack"
	"runtime"
	"strconv"
)

// hooks check
// set loglevel
// cache refresh
// update notify
// ... (Service's)

func BootInit(nodeType LookupConsts.ServiceNodeType) bool {
	lookUpClient := Lookup.GetNodeLookupClient()
	lookUpArgs := LookupArgs.GetLookupAppArgs()

	lookUpArgs.SetServiceNodeType(nodeType)
	if suc := lookUpArgs.ProcessAppArgs(); !suc {
		return false
	}

	Log.SetOutput(string(nodeType) + "." + lookUpArgs.Identifier + "_")

	// Max P
	Log.Criticalf("Number of cpu num[%v] \n", runtime.NumCPU())
	runtime.GOMAXPROCS(runtime.NumCPU())

	// client init
	if suc := lookUpClient.Init(lookUpArgs.NodeType, lookUpArgs.Identifier, lookUpArgs.NodeLookup); !suc {
		return false
	}

	return true
}

func AppInit(svcMapping []ApiService.PathMapping) bool {
	// set loglevel
	lookupEvent := LookupHook.GetEventRequest()
	lookupEvent.RegisterHook(string(LookupHook.NodeSetLogLevel),
		func(args []string) bool {
			if args != nil && len(args) == 1 {
				logLevel, err := strconv.Atoi(args[0])
				if err != nil {
					Log.Errorf("invalid log level [%v]\n", args[0])
					return false
				}
				Log.Criticalf("Set Loglevel : %d\n", logLevel)
				Log.SetLogLevel(logLevel)
				return true
			}
			Log.Errorf("invalid set log level args [%+v]\n", args)
			return false
		})

	// dump app stack
	lookupEvent.RegisterHook(string(LookupHook.NodeDumpAppStack),
		func(args []string) bool {
			lookupArgs := LookupArgs.GetLookupAppArgs()
			app := lookupArgs.AppName + "." + lookupArgs.Identifier
			r := Stack.DumpAppStack(app, false)
			Log.Criticalf("Dump App Stack result : %v\n", r)
			return false
		})

	lookUpClient := Lookup.GetNodeLookupClient()

	// update notify
	lookupEvent.RegisterHook(string(LookupHook.NodeUpdatedNotify),
		func([]string) bool { lookUpClient.RpcNodeUpdated(); return true })

	lookUpClient.SetEventRequestHook(LookupHook.GetEventRequest().EventRequest)
	ApiService.GetAppService().MergeMapping(lookUpClient.CreateMuxForLookup())
	ApiService.GetAppService().MergeMapping(svcMapping)

	lookUpArs := LookupArgs.GetLookupAppArgs()
	ApiService.GetAppService().StartHttpApi(lookUpArs.ServerHost, lookUpArs.BindAddrAny)

	// register nodes
	lookUpDs := lookUpClient.GetDataStore()
	regNodes := []LookupDS.ServiceNode{
		{Uid: lookUpDs.GetAppUid(), NodeType: lookUpDs.GetNodeType(), ApiRoot: lookUpArs.ServerHost},
	}

	// client register and updated
	if suc := lookUpClient.CreateClientUpdateHook(regNodes); !suc {
		return false
	}

	return true
}
