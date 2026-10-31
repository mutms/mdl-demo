# The CLI console's shell setup, for login shells (tmux starts them).
export PATH="$HOME/.local/bin:/usr/share/mdl-demo/cli/bin:$PATH"
# Keep Claude Code's fullscreen renderer, which works in tmux, but leave the mouse
# to the browser terminal, so selecting and Ctrl+C work as on any web page.
export CLAUDE_CODE_DISABLE_MOUSE=1
# Debian's Node 20 keeps the global WebSocket that browser/cdp.mjs needs behind a flag.
export NODE_OPTIONS=--experimental-websocket

# Once per session, not in every tmux window. Nothing here runs login(1), so
# /etc/motd would never be shown.
case "$-" in
    *i*) [ "${TMUX_PANE:-%0}" = "%0" ] && cat "/usr/share/mdl-demo/cli/motd${MDL_DEMO_EDITABLE_CODE:+-editable}" ;;
esac
