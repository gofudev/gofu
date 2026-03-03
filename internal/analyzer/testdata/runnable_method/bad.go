package runnable_method

type Server struct{}

//gofu:runnable
func (s *Server) Handle() string {
	return "ok"
}
