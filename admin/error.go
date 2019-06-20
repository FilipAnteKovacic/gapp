package main

import (
	"fmt"
	"os"
)

// HandleError handle error depends on ENV
func HandleError(proc ServiceLog, status string, err error, save bool) {

	if os.Getenv("DEBUG") == "true" {

		fmt.Println("------------")
		fmt.Println("----ERROR---")
		fmt.Println("------------")

		fmt.Println(proc)
		fmt.Println("------------")
		fmt.Println(status)
		fmt.Println("------------")
		fmt.Println(err)
		fmt.Println("------------")

	}

	if os.Getenv("PRINT_ERROR") == "true" {
		fmt.Println(status, err)
	}

	if save {

		proc.Status = "error"
		proc.Msg = status + ":" + err.Error()
		go SaveLog(proc)
	}

	return
}
