package LookupDS

import (
	uuid "github.com/satori/go.uuid"
	"github.com/tauruscorpius/appcommon/Lookup/LookupConsts"
	"sort"
	"strconv"
	"sync"
	"time"
)

type NodeQueryFilter struct {
	Exclude []string `json:"exclude,omitempty"`
	Include []string `json:"include,omitempty"`
}

func (t NodeQueryFilter) Kill(n string) bool {
	if t.Exclude != nil && len(t.Exclude) > 0 {
		for _, v := range t.Exclude {
			if v == n {
				return true
			}
		}
		return false
	}
	if t.Include != nil && len(t.Include) > 0 {
		for _, v := range t.Include {
			if v == n {
				return false
			}
		}
		return true
	}
	return false
}

type ServiceNode struct {
	Uid      string `json:"uid,omitempty"`
	NodeType string `json:"type,omitempty"`
	ApiRoot  string `json:"api-root,omitempty"`
	Scheme   string `json:"scheme,omitempty"` // https|http default https
}

func (t *ServiceNode) Valid() bool {
	return t.Uid != "" && t.NodeType != "" && t.ApiRoot != ""
}

func (t *ServiceNode) JoinUrl(path string) string {
	if t.Scheme != "http" {
		return "https://" + t.ApiRoot + path
	} else {
		return "http://" + t.ApiRoot + path
	}
}

type RegisterNode struct {
	ServiceNode
	ServedLookupUid string    `json:"served-Lookup-uid,omitempty"`
	CreateTime      time.Time `json:"create-time,omitempty"`
}

type MapRegisterNode struct {
	rw       sync.RWMutex
	regNodes map[string]RegisterNode
}

func (t *MapRegisterNode) Init() {
	t.regNodes = make(map[string]RegisterNode)
}

func (t *MapRegisterNode) Add(node RegisterNode) {
	t.rw.Lock()
	defer t.rw.Unlock()
	t.regNodes[node.Uid] = node
}

func (t *MapRegisterNode) erase(filter func(node *RegisterNode) bool) {
	t.rw.Lock()
	defer t.rw.Unlock()
	var delNode []string
	for k, v := range t.regNodes {
		if filter(&v) {
			delNode = append(delNode, k)
		}
	}
	for _, v := range delNode {
		delete(t.regNodes, v)
	}
}

func (t *MapRegisterNode) Sort() []*RegisterNode {
	t.rw.Lock()
	defer t.rw.Unlock()
	var resNode []*RegisterNode
	for _, v := range t.regNodes {
		resNode = append(resNode, &v)
	}
	// sort it
	sort.Slice(resNode, func(i, j int) bool { return resNode[i].Uid < resNode[j].Uid })
	return resNode
}

func (t *MapRegisterNode) SortWithFilter(uidFilter, typeFilter NodeQueryFilter) []RegisterNode {
	t.rw.Lock()
	defer t.rw.Unlock()
	var resNode []RegisterNode
	for _, v := range t.regNodes {
		if uidFilter.Kill(v.Uid) {
			continue
		}
		if typeFilter.Kill(v.NodeType) {
			continue
		}
		resNode = append(resNode, v)
	}
	// sort it
	sort.Slice(resNode, func(i, j int) bool { return resNode[i].Uid < resNode[j].Uid })
	return resNode
}

func (t *MapRegisterNode) Equal(n *RegisterNode) bool {
	for _, v := range t.regNodes {
		if n.Uid == v.Uid && n.ServedLookupUid == v.ServedLookupUid {
			return true
		}
	}
	return false
}

type RegisterNodes struct {
	applicationUid string
	nodeInfo       string
	nodeType       string
	lookUpNodes    []string
	Nodes          MapRegisterNode // using Uid as index
}

func (t *RegisterNodes) Init(nodeType LookupConsts.ServiceNodeType, identifier string, staticLookup []string) {
	// init
	t.Nodes.Init()

	// node id
	t.setNodeType(string(nodeType))

	// identifier
	t.setNodeInfo(identifier)

	// Uid
	id := uuid.NewV4()
	t.setAppUid(string(nodeType) + "-" + id.String())

	// static
	t.setStaticLookup(staticLookup)

}

func (t *RegisterNodes) setAppUid(uid string) {
	t.applicationUid = uid
}

func (t *RegisterNodes) GetAppUid() string {
	return t.applicationUid
}

func (t *RegisterNodes) setNodeType(nt string) {
	t.nodeType = nt
}

func (t *RegisterNodes) GetNodeType() string {
	return t.nodeType
}

func (t *RegisterNodes) setNodeInfo(ni string) {
	t.nodeInfo = ni
}

func (t *RegisterNodes) GetNodeInfo() string {
	return t.nodeInfo
}

func (t *RegisterNodes) setStaticLookup(lookup []string) {
	t.lookUpNodes = lookup
}

func (t *RegisterNodes) GetStaticLookupNode() []string {
	return t.lookUpNodes
}

func (t *RegisterNodes) FillLookupNodes() {
	for i, v := range t.lookUpNodes {
		t.Add(RegisterNode{
			ServiceNode: ServiceNode{
				Uid:      LookupConsts.StaticLookupNodeUidPrefix + strconv.Itoa(i),
				ApiRoot:  v,
				NodeType: string(LookupConsts.ServiceNodeTypeLookUp)}},
		)
	}
}

func (t *RegisterNodes) Add(node RegisterNode) {
	t.Nodes.Add(node)
}

func (t *RegisterNodes) Erase(filter func(node *RegisterNode) bool) {
	t.Nodes.erase(filter)
}

func (t *RegisterNodes) GetSort() []*RegisterNode {
	return t.Nodes.Sort()
}
