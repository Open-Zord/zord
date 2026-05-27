package main

import (
	_ "github.com/go-sql-driver/mysql"
	"go-skeleton/bootstrap"
	"go-skeleton/cmd/http/server"
)

// @title Swagger Zord API
// @version 1.0
// @description This is the Zord backend server.
func main() {
	reg, apiPrefix := bootstrap.Setup()
	serverInstance := server.NewServer(reg, apiPrefix)
	serverInstance.Start()
}
