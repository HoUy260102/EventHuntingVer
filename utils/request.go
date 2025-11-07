package utils

import (
	"fmt"
	"time"
)

func TotalFinishRequest(title string, collName string, method string, start time.Time) {
	fmt.Println(fmt.Sprintf("[%s]%s %s: %s-------\n", collName, method, title, time.Since(start).String()))
}
