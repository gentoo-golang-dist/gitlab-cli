
# Uses .gitlab-ci.yml in the current directory
glab ci dag

# Specify a different file
glab ci dag path/to/.gitlab-ci.yml

# Use the GitLab API to resolve include: directives
glab ci dag --compiled

# Output raw SVG to stdout
glab ci dag --output svg > pipeline.svg

# Hide implicit stage ordering edges
glab ci dag --stage-edges=false

# Simulate a branch push pipeline on main
glab ci dag --branch main

# Simulate a merge request pipeline
glab ci dag --source-branch feat/my-feature --target-branch main

# Simulate a tag/release pipeline
glab ci dag --tag v2.0.0

# Simulate a scheduled pipeline
glab ci dag --source schedule --branch main

# Simulate a web-triggered pipeline
glab ci dag --source web --branch main

# Set custom CI variables for simulation
glab ci dag --branch main --var DEPLOY_ENV=staging --var FEATURE_FLAG=true
