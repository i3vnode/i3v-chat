package basic

import (
	"errors"
	"io"
	"net/http"
)

// sendRequest
func sendRequest(url string, body io.Reader, addHeaders map[string]string, method string) (resp []byte, err error) {
	// 1. create req
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")

	// 2. set headers
	if len(addHeaders) > 0 {
		for k, v := range addHeaders {
			req.Header.Add(k, v)
		}
	}

	// 3. send http request
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		err = errors.New("http status err")
		//glog.Errorf("sendRequest failed, url=%v, response status code=%d", url, response.StatusCode)
		return
	}

	// 4. read response
	//resp, err = ioutil.ReadAll(response.Body)
	resp, err = io.ReadAll(response.Body)
	return
}
