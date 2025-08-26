package server

import (
	"runtime"

	"github.com/mark3labs/mcp-go/server"
	"github.com/cecep-91/excel-mcp-server/internal/tools"
)

type ExcelServer struct {
	server *server.MCPServer
}

func New(version string) *ExcelServer {
	s := &ExcelServer{}
	s.server = server.NewMCPServer(
		"excel-mcp-server",
		version,
	)
	if runtime.GOOS == "windows" {
		tools.AddExcelScreenCaptureTool(s.server)
	}
	return s
}

func (s *ExcelServer) Start() error {
	sseServer := server.NewSSEServer(s.server)
	return sseServer.Start(":8080")
}
