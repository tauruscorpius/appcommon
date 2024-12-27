package LookupConsts

const (
	StaticLookupNodeUidPrefix = "static_args_lookup_uid_"
)

type ServiceNodeType string

const (
	ServiceNodeTypeLookUp ServiceNodeType = "lookup"
)

const (
	DefaultHttpPingPath     = "/ping"
	DefaultEventRequestPath = "/service-node/event-request"
	DefaultPProfRequestPath = "/pprof"

	// Lookup Nodes Provide register and query Path

	LookupHttpRootPath       = "/lookup/api/v1"
	LookupHttpRegisterPath   = LookupHttpRootPath + "/register"
	LookupHttpDeRegisterPath = LookupHttpRootPath + "/deregister"
	LookupHttpNodeQueryPath  = LookupHttpRootPath + "/query"
)
