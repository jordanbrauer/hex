# A single Index handler + GET /users route
hex make:controller users

# Full RESTful CRUD
hex make:controller users --all

# A chosen subset of actions
hex make:controller users --actions index,show,store

# Preview the controller and route wiring without writing
hex make:controller users --all --dry-run
