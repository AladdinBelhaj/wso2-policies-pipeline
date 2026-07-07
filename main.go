package main

import (
	"fmt"
	"log"
	"os/exec"
	"wso2/scripts/vars"
)
func main() {

	vars.Load()

    cmd := exec.Command("curl", "-u", vars.Username + ":" + vars.Password, vars.BaseUrl + "/apis", "-k")
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}