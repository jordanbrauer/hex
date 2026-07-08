// Schema for hex/db configuration.

driver!: "sqlite" | "postgres"
dsn!:    string & !=""

pool?: {
	max_open_conns?:     int & >=0
	max_idle_conns?:     int & >=0
	conn_max_lifetime?:  string
	conn_max_idle_time?: string
}

pragmas?: [...string]
