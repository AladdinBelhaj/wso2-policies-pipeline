package main

import (
	"fmt"
)
func main() {

	fmt.Println("Hello" + username)



    cmd := exec.Command("curl", "-u", username + ":" + password, baseUrl + "/apis", "-k")
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}