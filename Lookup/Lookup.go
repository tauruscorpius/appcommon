package Lookup

import (
	"errors"
	"github.com/tauruscorpius/appcommon/ApiService"
	"github.com/tauruscorpius/appcommon/Consts"
	"github.com/tauruscorpius/appcommon/ExitHandler"
	"github.com/tauruscorpius/appcommon/HttpClient"
	"github.com/tauruscorpius/appcommon/Json"
	"github.com/tauruscorpius/appcommon/Log"
	"github.com/tauruscorpius/appcommon/Lookup/LookupConsts"
	"github.com/tauruscorpius/appcommon/Lookup/LookupDS"
	"github.com/tauruscorpius/appcommon/Lookup/RpcDS"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	NodeLookupRefreshQueueSize int = 1024
)

// NodeLookupClient a client of node Lookup
type NodeLookupClient struct {
	fetchLocker      sync.RWMutex
	ds               LookupDS.RegisterNodes
	RpcNodeUpdate    chan struct{} // etcd node updated
	eventRequestHook func(eventId string, eventArgs []string) bool
}

var (
	once             sync.Once
	nodeLookUpClient *NodeLookupClient
)

func GetNodeLookupClient() *NodeLookupClient {
	once.Do(func() {
		nodeLookUpClient = &NodeLookupClient{}
	})
	return nodeLookUpClient
}

// implementation

func (t *NodeLookupClient) Init(nodeType LookupConsts.ServiceNodeType, identifier string, staticLookup []string) bool {
	t.RpcNodeUpdate = make(chan struct{}, NodeLookupRefreshQueueSize)
	t.ds.Init(nodeType, identifier, staticLookup)
	return true
}

func (t *NodeLookupClient) GetDataStore() *LookupDS.RegisterNodes {
	return &t.ds
}

func (t *NodeLookupClient) CreateMuxForLookup() []ApiService.PathMapping {
	var v = []ApiService.PathMapping{
		{LookupConsts.DefaultHttpPingPath, t.CbMethodPing},
		{LookupConsts.DefaultEventRequestPath, t.CbMethodServiceEvent},
	}
	return v
}

func (t *NodeLookupClient) SetEventRequestHook(f func(eventId string, eventArgs []string) bool) {
	t.eventRequestHook = f
}

func (t *NodeLookupClient) RpcNodeUpdated() {
	t.RpcNodeUpdate <- struct{}{}
}

func (t *NodeLookupClient) CreateClientUpdateHook(regNodes []LookupDS.ServiceNode) bool {
	exitDrop := func() bool {
		for _, v := range regNodes {
			registerNode := &RpcDS.HttpRegisterRequest{
				ServiceNode: v,
			}
			_, _ = t.sendLookupHttpRequest("deregister+"+v.Uid, LookupConsts.LookupHttpDeRegisterPath, registerNode)
		}
		return true
	}
	// update Lookup node
	GetNodeLookupClient().GetDataStore().FillLookupNodes()
	go func() {
		ExitHandler.GetExitFuncChain().Add(exitDrop)
		exit := false
		for !exit {
			select {
			case <-time.After(time.Second):
				t.fetchAllRegisterNodes()
				// register current node
				for _, v := range regNodes {
					registerNode := &RpcDS.HttpRegisterRequest{
						ServiceNode: v,
					}
					_, _ = t.sendLookupHttpRequest("register+"+v.Uid, LookupConsts.LookupHttpRegisterPath, registerNode)
				}
			case <-t.RpcNodeUpdate:
				Log.Criticalf("Register nodes modified, update by node updated trigger\n")
				t.fetchAllRegisterNodes()
			case <-ExitHandler.GetExitFuncChain().AppContext.Done():
				Log.Criticalf("System exiting, do drop and clean work\n")
				exitDrop()
				exit = true
				break
			}
		}
	}()

	return true
}

func (t *NodeLookupClient) getCurrentRegisterNodes() (*LookupDS.MapRegisterNode, error) {
	registerNode := &RpcDS.HttpServiceQueryRequest{
		FromUid:   t.ds.GetAppUid(),
		UidFilter: LookupDS.NodeQueryFilter{Exclude: []string{t.ds.GetAppUid()}},
	}
	body, err := t.sendLookupHttpRequest("fetchRegisterNodes+", LookupConsts.LookupHttpNodeQueryPath, registerNode)
	if err != nil {
		Log.Errorf("error fetch latest Service nodes.\n")
		return nil, err
	}
	response := &RpcDS.HttpServiceQueryResponse{}
	err = Json.Unmarshal([]byte(body), response)
	if err != nil {
		Log.Errorf("error unmarshal latest Service nodes[%s].\n", body)
		return nil, err
	}
	if response.Nodes == nil {
		Log.Errorf("nil fetched latest Service nodes.\n")
		return nil, errors.New("error nil nodes fetched")
	}
	currentNodeMap := LookupDS.MapRegisterNode{}
	currentNodeMap.Init()
	for k, v := range response.Nodes {
		if !v.Valid() {
			Log.Errorf("invalid data %s Service node [%+v], key %s\n", v, registerNode, k)
			continue
		}
		t.ds.Add(v)
		currentNodeMap.Add(v)
	}
	return &currentNodeMap, nil
}

// fetchAllRegisterNodes update Service topology from naming server
func (t *NodeLookupClient) fetchAllRegisterNodes() bool {
	t.fetchLocker.Lock()
	defer t.fetchLocker.Unlock()

	currentNodeMap, err := t.getCurrentRegisterNodes()
	if err != nil {
		Log.Errorf("Get Current Register Node failed, err[%v]\n", err)
		return false
	}
	// erase expired node
	t.ds.Erase(func(n *LookupDS.RegisterNode) bool {
		if currentNodeMap == nil {
			return false
		}
		if currentNodeMap.Equal(n) {
			return false
		}
		return true
	})
	return true
}

func (t *NodeLookupClient) sendLookupHttpRequest(sender, path string, x interface{}) (string, error) {
	lookupList := t.ds.Nodes.SortWithFilter(
		LookupDS.NodeQueryFilter{}, LookupDS.NodeQueryFilter{Include: []string{string(LookupConsts.ServiceNodeTypeLookUp)}})
	if len(lookupList) == 0 {
		Log.Criticalf("httpRequest[%s]: object[%+v], cant not find any Lookup node using static Lookup fill [%+v]\n", sender, x, lookupList)
		nodeLookUpClient.ds.FillLookupNodes()
	}
	return t.sendTargetNode(sender, path, x, LookupConsts.ServiceNodeTypeLookUp, lookupList, true)
}

func (t *NodeLookupClient) SendServiceHttpRequest(sender, path string, targetSvcType LookupConsts.ServiceNodeType, x interface{}, readBody bool) (string, error) {
	targetList := t.ds.Nodes.SortWithFilter(
		LookupDS.NodeQueryFilter{}, LookupDS.NodeQueryFilter{Include: []string{string(targetSvcType)}})
	return t.sendTargetNode(sender, path, x, targetSvcType, targetList, readBody)
}

func (t *NodeLookupClient) sendTargetNode(sender, path string, x interface{}, targetType LookupConsts.ServiceNodeType, targetList []LookupDS.RegisterNode, readBody bool) (string, error) {
	if targetList == nil || len(targetList) == 0 {
		return "", errors.New("no target Service node found svcType " + string(targetType))
	}
	data, err := Json.Marshal(x)
	if err != nil {
		Log.Errorf("httpRequest[%s]: object[%+v] marshal failed\n", sender, x)
		return "", err
	}

	for _, v := range targetList {
		Log.Tracef("httpRequest[%s] -> remote[%v]: object[%+v] Request Body[%s]\n", sender, v, x, string(data))

		url := v.JoinUrl(path)
		statusCode, resp, err := HttpClient.PostHx(url, x, readBody)
		if err != nil {
			Log.Errorf("httpRequest[%s] url[%s] failed, object[%+v], err %v\n", sender, url, x, err)
		} else if statusCode != http.StatusOK {
			Log.Errorf("httpRequest[%s] url[%s] failed, object[%+v], status code %d\n", sender, url, x, statusCode)
		} else {
			Log.Tracef("httpRequest[%s] url[%s] succeed, object[%+v], status code %d\n", sender, url, x, statusCode)
			return resp, nil
		}
		if !strings.HasPrefix(v.Uid, LookupConsts.StaticLookupNodeUidPrefix) {
			nodeLookUpClient.ds.Erase(func(n *LookupDS.RegisterNode) bool {
				if v.Uid == n.Uid {
					Log.Criticalf("Erase Request failed - Node : %+v, url[%s]\n", n, url)
					return true
				}
				return false
			})
		}
		break
	}
	return "", errors.New("error all request failed, svcType " + string(targetType))
}

func (t *NodeLookupClient) CbMethodPing(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		Log.Tracef("receive POST data\n%s\n", body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		pingData := &RpcDS.HttpPingRequest{}
		err = Json.Unmarshal(body, pingData)
		if err != nil {
			Log.Debugf("Unmarshal failed : %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		Log.Tracef("Receive Ping from : %s\n", pingData.FromUid)

		if pingData.ToUid != GetNodeLookupClient().ds.GetAppUid() {
			Log.Errorf("Ping Target not this node, toUID[%s] appUid[%s]\n", pingData.ToUid, GetNodeLookupClient().ds.GetAppUid())
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		pingResponseData := &RpcDS.HttpPingResponse{ResponseUid: GetNodeLookupClient().ds.GetAppUid()}
		data, err := Json.Marshal(pingResponseData)
		if err != nil {
			Log.Debugf("Marshal failed : %v\n", err)
			_, err := w.Write([]byte(Consts.NullJson))
			if err != nil {
				Log.Errorf("write http response error : %v\n", err)
			}
			return
		}
		_, err = w.Write(data)
		if err != nil {
			Log.Errorf("write http response error : %v\n", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (t *NodeLookupClient) CbMethodServiceEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		Log.Debugf("receive POST data\n%s\n", body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		eventRequest := &RpcDS.HttpServiceEventRequest{}
		err = Json.Unmarshal(body, eventRequest)
		if err != nil {
			Log.Debugf("Unmarshal failed : %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		Log.Criticalf("Received Event Request : eventId[%s] Event Args[%+v]\n", eventRequest.EventId, eventRequest.EventArgs)

		result := t.eventRequestHook(eventRequest.EventId, eventRequest.EventArgs)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		eventRequestResult := &RpcDS.HttpServiceEventResponse{HttpDefaultResponse: RpcDS.HttpDefaultResponse{Result: result}}
		data, err := Json.Marshal(eventRequestResult)
		if err != nil {
			Log.Debugf("Marshal failed : %v\n", err)
			_, err := w.Write([]byte(Consts.NullJson))
			if err != nil {
				Log.Errorf("write http response error : %v\n", err)
			}
			return
		}
		_, err = w.Write(data)
		if err != nil {
			Log.Errorf("write http response error : %v\n", err)
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
