
# Uses .gitlab-ci.yml in the current directory
glab ci visualize

# Specify a different file
glab ci visualize path/to/.gitlab-ci.yml

# Open an interactive pan/zoom view via a local HTTP server
glab ci visualize --web

# Output raw SVG to stdout
glab ci visualize --output svg > pipeline.svg
