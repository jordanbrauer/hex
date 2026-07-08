driver: "sqlite" | "postgres"
dsn:    string & !=""
pool?: {
	max_open_conns?: int & >=0
	max_idle_conns?: int & >=0
}
