package api

import (
	"fmt"
	"net/http"
	"testing"
)

const (
	host = "localhost:8888"
)

func TestAPIClientRequestStore(t *testing.T) {
	lm := NewLearningMaterialAPI()
	go http.ListenAndServe(host, lm.Handler())

	resp, err := http.Get("http://"+host+"/api/tag1")
	resp, err = http.Get("http://"+host+"/api/tag2")
	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	//body, err := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	t.Fatal(err)
	//}

	// Maybe used just for testing purpose

	clients := make([]*ClientRequest, len(lm.crs.clientsR))
	var i int
	for _, c := range lm.crs.clientsR {
		clients[i] = c
		i += 1
	}

	fmt.Println(clients[0].requestsMade)


}
