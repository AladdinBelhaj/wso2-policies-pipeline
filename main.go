package main

import (
	"fmt"
	"log"
	"os/exec"
	"wso2/scripts/envvars"
)
func main() {

	envvars.Load()

    cmd := exec.Command("curl", "-u", envvars.Username + ":" + envvars.Password, envvars.BaseUrl + "/apis", "-k")
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}