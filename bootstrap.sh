#!/bin/bash
#
# Bootstrap Script for Omnivex Combine Project
#
# This script ensures that the latest versions of Go and Task are installed.
# It installs Go and Task if they are not present or updates them to the latest versions.
# Finally, it installs project dependencies using Task.

set -euo pipefail
set -x
#######################################
# Prints informational messages to STDOUT.
# Globals:
#   None
# Arguments:
#   Message to print.
# Outputs:
#   Informational message to STDOUT.
# Returns:
#   None
#######################################
info() {
  echo "[INFO] $*"
}

#######################################
# Prints error messages to STDERR with a timestamp.
# Globals:
#   None
# Arguments:
#   Message to print.
# Outputs:
#   Error message to STDERR.
# Returns:
#   None
#######################################
err() {
  echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')]: $*" >&2
}

#######################################
# Compares two semantic versions.
# Returns 0 (true) if $1 >= $2, else 1 (false).
# Globals:
#   None
# Arguments:
#   $1: Version to check if greater than or equal to $2.
#   $2: Reference version.
# Outputs:
#   None
# Returns:
#   0 if $1 >= $2, 1 otherwise.
#######################################
version_ge() {
  [[ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]
}

#######################################
# Fetches the latest Task release tag from GitHub.
# Globals:
#   None
# Arguments:
#   None
# Outputs:
#   Latest Task version tag to STDOUT.
# Returns:
#   None
#######################################
get_latest_task_version() {
  curl -s https://api.github.com/repos/go-task/task/releases/latest | \
    grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

#######################################
# Fetches the latest Go version from the official Go website.
# Globals:
#   None
# Arguments:
#   None
# Outputs:
#   Latest Go version to STDOUT.
# Returns:
#   None
#######################################
get_latest_go_version() {
  curl -s https://go.dev/VERSION?m=text
}

#######################################
# Detects the operating system and architecture.
# Sets PLATFORM and ARCH variables accordingly.
# Globals:
#   PLATFORM
#   ARCH
# Arguments:
#   None
# Outputs:
#   None
# Returns:
#   Exits if OS or ARCH is unsupported.
#######################################
detect_system() {
  local os arch

  os="$(uname)"
  arch="$(uname -m)"

  case "${os}" in
    Linux)
      PLATFORM="Linux"
      ;;
    Darwin)
      PLATFORM="Darwin"
      ;;
    *)
      err "Unsupported OS: ${os}"
      exit 1
      ;;
  esac

  case "${arch}" in
    x86_64|amd64)
      ARCH="amd64"
      ;;
    arm64|aarch64)
      ARCH="arm64"
      ;;
    *)
      err "Unsupported architecture: ${arch}"
      exit 1
      ;;
  esac
}

#######################################
# Installs Go by downloading the latest version.
# Globals:
#   PLATFORM
#   ARCH
# Arguments:
#   None
# Outputs:
#   Installation messages to STDOUT.
# Returns:
#   Exits if installation fails.
#######################################
install_go() {
  info "Installing the latest version of Go..."

  local latest_go_version go_tarball_url temp_dir

  latest_go_version="$(get_latest_go_version)"

  if [[ -z "${latest_go_version}" ]]; then
    err "Failed to fetch the latest Go version."
    exit 1
  fi

  info "Latest Go version: ${latest_go_version}"

  # Adjust the URL based on OS
  if [[ "${PLATFORM}" == "Linux" ]]; then
    go_tarball_url="https://golang.org/dl/${latest_go_version}.linux-${ARCH}.tar.gz"
  elif [[ "${PLATFORM}" == "Darwin" ]]; then
    go_tarball_url="https://golang.org/dl/${latest_go_version}.darwin-${ARCH}.tar.gz"
  fi

  info "Downloading Go from ${go_tarball_url}..."

  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' EXIT

  cd "${temp_dir}"

  if ! curl -sSL "${go_tarball_url}" -o "go.tar.gz"; then
    err "Failed to download Go from ${go_tarball_url}"
    exit 1
  fi

  # Verify the downloaded file is a valid tar.gz
  if ! tar -tzf "go.tar.gz" > /dev/null 2>&1; then
    err "Downloaded Go archive is not a valid tar.gz file."
    exit 1
  fi

  if ! sudo tar -C /usr/local -xzf "go.tar.gz"; then
    err "Failed to extract Go archive to /usr/local."
    exit 1
  fi

  # Ensure /usr/local/go/bin is in PATH
  if [[ ":$PATH:" != *":/usr/local/go/bin:"* ]]; then
    info "Adding /usr/local/go/bin to PATH."
    echo 'export PATH="/usr/local/go/bin:$PATH"' >> "${HOME}/.bashrc"
    echo 'export PATH="/usr/local/go/bin:$PATH"' >> "${HOME}/.zshrc"
    info "Added /usr/local/go/bin to PATH in ~/.bashrc and ~/.zshrc. Please reload your shell."
  else
    info "/usr/local/go/bin is already in PATH."
  fi

  trap - EXIT

  info "Go ${latest_go_version} installation completed."
}

#######################################
# Fetches the download URL for the latest Task release matching the OS and architecture.
# Globals:
#   PLATFORM
#   ARCH
# Arguments:
#   None
# Outputs:
#   Download URL to STDOUT.
# Returns:
#   None
#######################################
get_task_download_url() {
  local latest_version os arch download_url

  latest_version="$(get_latest_task_version)"
  os="$(echo "${PLATFORM}" | tr '[:upper:]' '[:lower:]')"
  arch="$(echo "${ARCH}" | tr '[:upper:]' '[:lower:]')"

  # Fetch assets JSON
  local assets_json
  assets_json="$(curl -s https://api.github.com/repos/go-task/task/releases/tags/${latest_version})"

  # Extract the download URL for the matching asset
  download_url="$(echo "${assets_json}" | grep '"browser_download_url":' | grep "${os}" | grep "${arch}" | head -n1 | sed -E 's/.*"([^"]+)".*/\1/')"

  if [[ -z "${download_url}" || "${download_url}" == "null" ]]; then
    err "Failed to find download URL for Task ${latest_version} on ${PLATFORM} ${ARCH}."
    exit 1
  fi

  echo "${download_url}"
}

#######################################
# Installs Task by downloading the latest version.
# Globals:
#   PLATFORM
#   ARCH
# Arguments:
#   None
# Outputs:
#   Installation messages to STDOUT.
# Returns:
#   Exits if installation fails.
#######################################
install_task() {
  info "Installing the latest version of Task..."

  local latest_task_version task_download_url temp_dir

  latest_task_version="$(get_latest_task_version)"

  if [[ -z "${latest_task_version}" ]]; then
    err "Failed to fetch the latest Task version."
    exit 1
  fi

  info "Latest Task version: ${latest_task_version}"

  task_download_url="$(get_task_download_url)"

  info "Downloading Task from ${task_download_url}..."

  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' EXIT

  cd "${temp_dir}"

  local package_name
  package_name="$(basename "${task_download_url}")"

  if ! curl -sSL "${task_download_url}" -o "${package_name}"; then
    err "Failed to download Task from ${task_download_url}"
    exit 1
  fi

  # Handle different package formats
  if [[ "${package_name}" == *.deb ]]; then
    if ! sudo dpkg -i "${package_name}"; then
      err "Failed to install Task .deb package"
      exit 1
    fi
  elif [[ "${package_name}" == *.tar.gz ]]; then
    if ! tar -xzf "${package_name}"; then
      err "Failed to extract Task archive"
      exit 1
    fi

    if [[ ! -f "task" ]]; then
      err "Task binary not found in the archive"
      exit 1
    fi

    chmod +x task
    if [[ -w "/usr/local/bin" ]]; then
      sudo mv task "/usr/local/bin/"
    else
      mkdir -p "${HOME}/bin"
      mv task "${HOME}/bin/"
    fi
  else
    err "Unsupported package format: ${package_name}"
    exit 1
  fi

  trap - EXIT

  info "Task ${latest_task_version} installation completed."
}

#######################################
# Checks if Go is installed and installs the latest version if not.
# Globals:
#   PLATFORM
#   ARCH
# Arguments:
#   None
# Outputs:
#   Go installation and version information to STDOUT.
# Returns:
#   None
#######################################
check_go_installation() {
  info "==========================================="
  info "Checking for Go installation..."
  info "==========================================="

  if ! command -v go &> /dev/null; then
    info "Go not found."
    install_go
  else
    local installed_version latest_go_version

    installed_version="$(go version | awk '{print $3}' | sed 's/go//')"
    latest_go_version="$(get_latest_go_version)"

    if [[ -z "${latest_go_version}" ]]; then
      err "Failed to fetch the latest Go version."
      exit 1
    fi

    if version_ge "${latest_go_version}" "${installed_version}"; then
      info "Go is up-to-date (version ${installed_version})."
    else
      info "Go version ${installed_version} is outdated. Updating to ${latest_go_version}."
      install_go
    fi
  fi
}

#######################################
# Checks if Task is installed and installs the latest version if not.
# Globals:
#   PLATFORM
#   ARCH
# Arguments:
#   None
# Outputs:
#   Task installation and version information to STDOUT.
# Returns:
#   None
#######################################
check_task_installation() {
  info "==========================================="
  info "Checking for Task installation..."
  info "==========================================="

  if ! command -v task &> /dev/null; then
    info "Task not found."
    install_task
  else
    local current_version latest_task_version

    current_version="$(task --version | awk '{print $2}')"
    latest_task_version="$(get_latest_task_version)"

    if [[ -z "${latest_task_version}" ]]; then
      err "Failed to fetch the latest Task version."
      exit 1
    fi

    if version_ge "${latest_task_version}" "${current_version}"; then
      info "Task is up-to-date (version ${current_version})."
    else
      info "Task version ${current_version} is outdated. Updating to ${latest_task_version}."
      install_task
    fi
  fi
}

#######################################
# Main function that orchestrates the bootstrap process.
# Globals:
#   PLATFORM
#   ARCH
# Arguments:
#   All script arguments.
# Outputs:
#   Bootstrap status messages to STDOUT.
# Returns:
#   None
#######################################
main() {
  detect_system
  check_go_installation
  check_task_installation

  info "==========================================="
  info "Bootstrap complete!"
  info "==========================================="
}

# Execute the main function with all script arguments
main "$@"
