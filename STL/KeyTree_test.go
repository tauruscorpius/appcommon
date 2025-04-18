package STL

import (
	"fmt"
	"testing"
)

func TestKeyTree_Fetch(t *testing.T) {
	var keytree KeyTree
	err := keytree.AddNode(15, []byte("123456789ABCDEF"), struct{}{})
	if err != nil {
		t.Fatalf("keytree.AddNode failed : %+v\n", err)
	}
	err, value := keytree.GetNode([]byte("123456789ABCDEF"))
	if err != nil {
		t.Errorf("keytree.GetNode failed : %+v\n", err)
	} else {
		fmt.Printf("keytree.GetNode succeed : %+v\n", value)
	}
}
