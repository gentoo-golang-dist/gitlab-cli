
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
