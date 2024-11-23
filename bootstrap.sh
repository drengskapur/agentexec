#!/bin/bash
#
# Bootstrap script for installing Task (https://taskfile.dev).

set -euo pipefail

readonly INSTALL_SCRIPT="https://raw.githubusercontent.com/go-task/task/main/install-task.sh"
readonly SHELL_RC_FILES=("${HOME}/.bashrc" "${HOME}/.zshrc")

#######################################
# Print information to stdout.
# Globals:
#   None
# Arguments:
#   Any string to print as information.
#######################################
info() {
  echo "[INFO] $*"
}

#######################################
# Add directory to PATH in shell rc files if not already added.
# Globals:
#   SHELL_RC_FILES
# Arguments:
#   Directory to add to PATH.
#######################################
add_to_path() {
  local dir="$1"
  local rc_file

  for rc_file in "${SHELL_RC_FILES[@]}"; do
    if [[ -f "$rc_file" ]] && ! grep -q "export PATH=\"$dir:\$PATH\"" "$rc_file"; then
      echo "export PATH=\"$dir:\$PATH\"" >> "$rc_file"
      info "Added $dir to PATH in $rc_file"
    fi
  done
}

#######################################
# Install Task and add it to PATH.
# Globals:
#   INSTALL_SCRIPT
# Arguments:
#   None
#######################################
install_task() {
  info "Installing Task..."
  local install_dir="${HOME}/.local/bin"
  mkdir -p "$install_dir"

  if ! curl -fsSL "$INSTALL_SCRIPT" | sh -s -- -d -b "$install_dir"; then
    info "Task installation failed."
    exit 1
  fi

  if [[ ":$PATH:" != *":$install_dir:"* ]]; then
    add_to_path "$install_dir"
  fi
}

#######################################
# Configure shell completions for Task.
# Globals:
#   None
# Arguments:
#   None
#######################################
setup_completions() {
  local shell_type
  shell_type="$(basename "$SHELL")"

  case "$shell_type" in
    bash)
      echo 'eval "$(task --completion bash)"' >> "${HOME}/.bashrc"
      info "Added bash completions"
      ;;
    zsh)
      echo 'eval "$(task --completion zsh)"' >> "${HOME}/.zshrc"
      info "Added zsh completions"
      ;;
    fish)
      mkdir -p "${HOME}/.config/fish/completions"
      task --completion fish > "${HOME}/.config/fish/completions/task.fish"
      info "Added fish completions"
      ;;
    *)
      info "Shell completions not configured for unsupported shell: $shell_type"
      ;;
  esac
}

#######################################
# Main script execution.
# Globals:
#   None
# Arguments:
#   None
#######################################
main() {
  if ! command -v task &>/dev/null; then
    install_task
    setup_completions
  else
    info "Task is already installed."
  fi

  if [[ -f "Taskfile.yml" ]]; then
    task setup
  fi
}

main "$@"
