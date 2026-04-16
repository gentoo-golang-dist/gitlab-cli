
# Uses .gitlab-ci.yml in the current directory
glab ci visualize

# Specify a different file
glab ci visualize path/to/.gitlab-ci.yml

# Open an interactive pan/zoom view via a local HTTP server
glab ci visualize --web

# Output raw SVG to stdout
glab ci visualize --output svg > pipeline.svg

# Hide implicit stage ordering edges
glab ci visualize --stage-edges=false

# Simulate a branch push pipeline on main
glab ci visualize --branch main

# Simulate a merge request pipeline
glab ci visualize --source-branch feat/my-feature --target-branch main

# Simulate a tag/release pipeline
glab ci visualize --tag v2.0.0

# Simulate a scheduled pipeline
glab ci visualize --source schedule --branch main

# Simulate a web-triggered pipeline
glab ci visualize --source web --branch main

# Set custom CI variables for simulation
glab ci visualize --branch main --var DEPLOY_ENV=staging --var FEATURE_FLAG=true
