package chess_server

import "github.com/labstack/echo/v4"

type ChessServer struct {
	MatchMaking MatchMaking
}

func (s *ChessServer) Init() {
	s.MatchMaking.Init()
}

type ChessServerContext struct {
	echo.Context
	Server *ChessServer
}
