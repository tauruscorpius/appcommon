package RpcDS

import (
	"github.com/tauruscorpius/appcommon/Lookup/LookupDS"
)

type HttpDefaultResponse struct {
	Result bool   `json:"result,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

type HttpPingRequest struct {
	FromNodeType string `json:"from-node-type,omitempty"`
	FromUid      string `json:"from-uid,omitempty"`
	ToUid        string `json:"to-uid,omitempty"`
}

type HttpPingResponse struct {
	ResponseUid string `json:"from-uid,omitempty"`
}

type HttpRegisterRequest struct {
	LookupDS.ServiceNode
}

type HttpServiceNode struct {
	Nodes []LookupDS.RegisterNode `json:"nodes,omitempty"`
}

type HttpServiceQueryResponse struct {
	HttpServiceNode
}

type HttpServiceQueryRequest struct {
	FromUid    string                   `json:"from-uid,omitempty"`
	UidFilter  LookupDS.NodeQueryFilter `json:"uid-filter,omitempty"`
	TypeFilter LookupDS.NodeQueryFilter `json:"type-filter,omitempty"`
}

// naming server -> service node

// HttpServicePushRequest notify service node to retrieve new node list
type HttpServicePushRequest struct {
	FromUid string `json:"from-uid,omitempty"` // uid of notify node Lookup
}

type HttpServicePushResponse struct {
	HttpDefaultResponse
}

type HttpServiceSetLogLevelRequest struct {
	FromUid  string `json:"from-uid,omitempty"` // uid of notify node Lookup
	LogLevel int    `json:"log-level,omitempty"`
}

type HttpServiceSetLogLevelResponse struct {
	HttpDefaultResponse
}

type HttpServiceEventRequest struct {
	FromUid   string   `json:"from-uid,omitempty"` // uid of notify node Lookup
	EventId   string   `json:"event-id,omitempty"`
	EventArgs []string `json:"event-args,omitempty"`
}

type HttpServiceEventResponse struct {
	HttpDefaultResponse
}
