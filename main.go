package main

import (
	"time"
)

func main() {
	SetUseTLS(false)
	Setup(false)

	time.Sleep(time.Second * 100)
}
