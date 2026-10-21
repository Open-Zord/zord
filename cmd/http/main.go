package main

import (
	httpboot "github.com/Open-Zord/zord/bootstrap/http"
	"github.com/Open-Zord/zord/cmd/http/server"
	_ "github.com/go-sql-driver/mysql"
)

// @title Swagger Zord API
// @version 1.0
// @description This is the Zord backend server.
func main() {
	reg, apiPrefix := httpboot.Setup()
	serverInstance := server.NewServer(reg, apiPrefix)
	serverInstance.Start()
}
