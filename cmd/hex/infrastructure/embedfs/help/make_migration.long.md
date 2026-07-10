Generate a timestamped migration pair at
`database/migrations/<timestamp>_<name>.{up,down}.sql`.

The timestamp is in the format golang-migrate expects (`yyyyMMddHHmmss`),
lexically sortable and unique to the second. When the name follows the
`create_<table>_table` convention the stubs are pre-filled with a matching
`CREATE TABLE` / `DROP TABLE`; edit them to add your real schema.
