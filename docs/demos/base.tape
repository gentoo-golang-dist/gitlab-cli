# Shared styling for glab demo tapes. Source this at the top of any tape to
# inherit the canonical look. This file contains only `Set` directives — it
# doesn't record anything on its own and isn't meant to be run directly.

# Terminal dimensions
Set Shell "bash"
Set Width 1400
Set Height 800

# Typography — bigger font, generous line height for readability when embedded
# in docs at reduced size.
Set FontSize 16
Set LineHeight 1.4
Set Padding 30

# Subtle dark grey frame with rounded corners and a macOS-style window bar —
# turns the recording into a polished screenshot without competing for attention.
Set Margin 10
Set MarginFill "#2B2A2F"
Set BorderRadius 12
Set WindowBar Colorful

# GitLab-flavored dark theme. ANSI roles are mapped to brand colors so
# `glab` status indicators (open/closed/merged) read on-brand.
Set Theme { "name": "GitLab Dark", "black": "#1F1E24", "red": "#E24329", "green": "#25B86F", "yellow": "#FCA326", "blue": "#428FDC", "magenta": "#FC6D26", "cyan": "#2E9BB8", "white": "#DCDCDE", "brightBlack": "#525058", "brightRed": "#EE6352", "brightGreen": "#2DA160", "brightYellow": "#F2C94C", "brightBlue": "#6E9EE6", "brightMagenta": "#FCA269", "brightCyan": "#66B8CC", "brightWhite": "#FAFAFA", "background": "#1F1E24", "foreground": "#E4E1E6", "selection": "#3E3B45", "cursor": "#E4E1E6" }

# Recording behavior
Set Framerate 24
Set TypingSpeed 60ms
Set WaitTimeout 30s
Set CursorBlink true
