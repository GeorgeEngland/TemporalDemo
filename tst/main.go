package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func main() {
	cl := http.Client{}
	reqParams := url.Values{}
	reqParams.Add("message", "Helopwd")
	apiUrl := "https://hiring.api.synthesia.io/crypto/sign" + "?" + reqParams.Encode()
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Authorization", "6a8ea49677f1aff14d1a35364d61520e")

	res, err := cl.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res.StatusCode)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(body))

}
