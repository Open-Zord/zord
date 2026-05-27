package main

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/Open-Zord/zord/bootstrap"
	"github.com/Open-Zord/zord/cmd/http/server"
)

// @title Swagger Zord API
// @version 1.0
// @description This is the Zord backend server.
func main() {
	reg, apiPrefix := bootstrap.Setup()
	serverInstance := server.NewServer(reg, apiPrefix)
	serverInstance.Start()
}
