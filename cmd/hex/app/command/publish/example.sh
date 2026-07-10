# Publish the web server's default config into config/
hex publish web

# Publish every framework provider's configs
hex publish --all

# Overwrite configs you have already published
hex publish web --force
