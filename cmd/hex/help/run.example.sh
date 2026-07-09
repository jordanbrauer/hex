# Run a Teal script (language inferred from the extension)
hex run script.tl

# Run inline Lua
hex run -c 'print("hello from lua")'

# Run inline Fennel
hex run -c '(print "hello")' --lang fnl

# Pipe a script in via stdin
echo 'print(1 + 1)' | hex run -

# Type-check a Teal file without running it
hex run script.tl --check
