package main

var UserMap = map[string]string{}
var LoginSession = map[string]chan struct{}{}

func init() {
	UserMap["markity"] = "12345"
	UserMap["mary"] = "56789"
}
