#!/bin/bash
#
# Bootstrap Script for Omnivex Project
#
# This script ensures that the latest versions of Go and Task are installed.
# It installs Go and Task if they are not present or updates them to the latest versions.

# shellcheck disable=SC2034  # Variables may appear unused but are used in trap
# shellcheck disable=SC2086  # Word splitting is intentional for some commands

set -euo pipefail

#######################################
# Global Constants and Environment Variables
#######################################
readonly ORIGINAL_DIR="${PWD}"
readonly SHELL_RC_FILES=("${HOME}/.bashrc" "${HOME}/.zshrc")
readonly DEFAULT_CURL_OPTS=(-fsSL --connect-timeout 10 --max-time 30)
readonly GO_DOWNLOAD_URL="https://go.dev/dl"
readonly TASK_REPO_URL="https://api.github.com/repos/go-task/task"

# Global variables (modified by functions)
declare PLATFORM=""
declare ARCH=""
declare -a TEMP_DIRS=()

#######################################
# Error handler for unexpected failures
# Arguments:
#   Exit code from failed command
#   Line number of failure
#######################################
err_handler() {
  local exit_code="${1}"
  local line_no="${2}"
  log_error "Error on line ${line_no}: Exit code ${exit_code}"
}
trap 'err_handler ${?} ${LINENO}' ERR

#######################################
# Cleanup handler for temporary resources
# Globals:
#   ORIGINAL_DIR
#   TEMP_DIRS
#######################################
cleanup() {
  local dir
  for dir in "${TEMP_DIRS[@]}"; do
    if [[ -d "${dir}" ]]; then
      rm -rf "${dir}"
      log_info "Cleaned up temporary directory: ${dir}"
    fi
  done
  cd "${ORIGINAL_DIR}" || log_error "Failed to return to original directory"
  log_info "Returned to original directory: ${ORIGINAL_DIR}"
}
trap cleanup EXIT

#######################################
# Log informational messages
# Arguments:
#   Message to log
#######################################
log_info() {
  echo -e "[INFO] $(date +'%Y-%m-%dT%H:%M:%S%z') - ${*}"
}

#######################################
# Log warning messages
# Arguments:
#   Message to log
#######################################
log_warn() {
  echo -e "[WARN] $(date +'%Y-%m-%dT%H:%M:%S%z') - ${*}" >&2
}

#######################################
# Log error messages and exit
# Arguments:
#   Message to log
#######################################
log_error() {
  echo -e "[ERROR] $(date +'%Y-%m-%dT%H:%M:%S%z') - ${*}" >&2
  exit 1
}

#######################################
# Version comparison
# Arguments:
#   $1: Version to check if greater than or equal to $2
#   $2: Reference version
# Returns:
#   0 if VERSION_A >= VERSION_B, 1 otherwise
#######################################
version_ge() {
  [[ "$(printf '%s\n' "${2}" "${1}" | sort -V | head -n1)" == "${2}" ]]
}

#######################################
# Get latest Task version
# Outputs:
#   Latest Task version tag to STDOUT
#######################################
get_latest_task_version() {
  local version_json
  local version_tag

  version_json="$(curl "${DEFAULT_CURL_OPTS[@]}" "${TASK_REPO_URL}/releases/latest")"
  version_tag="$(echo "${version_json}" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"

  if [[ -z "${version_tag}" ]]; then
    log_error "Failed to get latest Task version"
  fi
  echo "${version_tag}"
}

#######################################
# Get Latest Go Version
# Returns:
#   Latest Go version string.
#######################################
get_latest_go_version() {
  local version_output
  version_output="$(curl -s https://go.dev/VERSION?m=text | head -n1)"
  
  # Validate the version format
  if [[ "${version_output}" =~ ^go[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "$version_output"
  else
    log_error "Invalid Go version format retrieved: ${version_output}"
    exit 1
  fi
}

#######################################
# Detect system architecture and platform
# Globals Modified:
#   PLATFORM
#   ARCH
#######################################
detect_system() {
  local os
  local arch

  os="$(uname)"
  arch="$(uname -m)"

  case "${os}" in
    Linux)
      PLATFORM="linux"
      ;;
    Darwin)
      PLATFORM="darwin"
      ;;
    *)
      log_error "Unsupported OS: ${os}"
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
      log_error "Unsupported architecture: ${arch}"
      ;;
  esac

  readonly PLATFORM
  readonly ARCH
}

#######################################
# Add directory to PATH in shell rc files
# Arguments:
#   $1: Directory to add to PATH
#######################################
add_to_path() {
  local dir="${1}"
  local rc_file

  for rc_file in "${SHELL_RC_FILES[@]}"; do
    if [[ -f "${rc_file}" ]]; then
      if ! grep -Fxq "export PATH=\"${dir}:\$PATH\"" "${rc_file}"; then
        echo "export PATH=\"${dir}:\$PATH\"" >> "${rc_file}"
        log_info "Added ${dir} to PATH in ${rc_file}"
      else
        log_info "${dir} is already in PATH in ${rc_file}"
      fi
    fi
  done
}

#######################################
# Install Go
# Arguments:
#   $1: Go version to install
#######################################
install_go() {
  local go_version="${1}"
  local temp_dir
  local -a curl_cmd
  local -a tar_cmd

  log_info "Installing Go ${go_version}..."
  
  temp_dir="$(mktemp -d)"
  TEMP_DIRS+=("${temp_dir}")

  (
    if ! cd "${temp_dir}"; then
      log_error "Failed to enter temporary directory: ${temp_dir}"
    fi

    curl_cmd=("${DEFAULT_CURL_OPTS[@]}" 
              "${GO_DOWNLOAD_URL}/${go_version}.${PLATFORM}-${ARCH}.tar.gz" 
              -o "go.tar.gz")
    if ! curl "${curl_cmd[@]}"; then
      log_error "Failed to download Go"
    fi

    tar_cmd=(sudo tar -C /usr/local -xzf "go.tar.gz")
    if ! "${tar_cmd[@]}"; then
      log_error "Failed to extract Go to /usr/local"
    fi

    log_info "Extracted Go to /usr/local"
  )

  add_to_path "/usr/local/go/bin"
  log_info "Go ${go_version} installation completed"
}

#######################################
# Get Task download URL
# Arguments:
#   $1: Task version
# Outputs:
#   Download URL to STDOUT
#######################################
get_task_download_url() {
  local task_version="${1}"
  local assets_json
  local download_url

  assets_json="$(curl "${DEFAULT_CURL_OPTS[@]}" "${TASK_REPO_URL}/releases/tags/${task_version}")"
  download_url="$(echo "${assets_json}" | grep '"browser_download_url":' | \
    grep "${PLATFORM}" | grep "${ARCH}" | head -n1 | sed -E 's/.*"([^"]+)".*/\1/')"

  if [[ -z "${download_url}" || "${download_url}" == "null" ]]; then
    log_error "Failed to find download URL for Task ${task_version}"
  fi

  echo "${download_url}"
}

#######################################
# Install Task
# Arguments:
#   $1: Task version to install
#######################################
install_task() {
  local task_version="${1}"
  local download_url
  local package_name
  local temp_dir
  local -a curl_cmd
  local -a install_cmd

  log_info "Installing Task ${task_version}..."

  download_url="$(get_task_download_url "${task_version}")"
  log_info "Downloading Task from ${download_url}..."

  temp_dir="$(mktemp -d)"
  TEMP_DIRS+=("${temp_dir}")

  (
    if ! cd "${temp_dir}"; then
      log_error "Failed to enter temporary directory: ${temp_dir}"
    fi

    package_name="$(basename "${download_url}")"
    curl_cmd=("${DEFAULT_CURL_OPTS[@]}" "${download_url}" -o "${package_name}")
    
    if ! curl "${curl_cmd[@]}"; then
      log_error "Failed to download Task"
    fi

    if [[ "${package_name}" == *.deb ]]; then
      install_cmd=(sudo dpkg -i "${package_name}")
      if ! "${install_cmd[@]}"; then
        log_error "Failed to install Task .deb package"
      fi
      log_info "Installed Task using .deb package"
    elif [[ "${package_name}" == *.tar.gz ]]; then
      if ! tar -xzf "${package_name}"; then
        log_error "Failed to extract Task archive"
      fi
      log_info "Extracted Task archive"

      if [[ -f "task" ]]; then
        chmod +x task
        if [[ -w "/usr/local/bin" ]]; then
          sudo mv task "/usr/local/bin/"
          log_info "Moved Task to /usr/local/bin"
        else
          mkdir -p "${HOME}/bin"
          mv task "${HOME}/bin/"
          log_info "Moved Task to ${HOME}/bin"
          add_to_path "${HOME}/bin"
        fi
      else
        log_error "Task binary not found in the archive"
      fi
    else
      log_error "Unsupported package format: ${package_name}"
    fi
  )

  log_info "Task ${task_version} installation completed"
}

#######################################
# Check Go installation
#######################################
check_go_installation() {
  local installed_version
  local latest_go_version

  log_info "==========================================="
  log_info "Checking for Go installation..."
  log_info "==========================================="

  if ! command -v go &> /dev/null; then
    log_info "Go not found"
    latest_go_version="$(get_latest_go_version)"
    install_go "${latest_go_version}"
  else
    installed_version="$(go version | awk '{print $3}' | sed 's/go//')"
    latest_go_version="$(get_latest_go_version)"

    if version_ge "${installed_version}" "${latest_go_version}"; then
      log_info "Go is up-to-date (version ${installed_version})"
    else
      log_info "Go version ${installed_version} is outdated. Updating to ${latest_go_version}"
      install_go "${latest_go_version}"
    fi
  fi
}

#######################################
# Check Task installation
#######################################
check_task_installation() {
  local current_version
  local latest_task_version

  log_info "==========================================="
  log_info "Checking for Task installation..."
  log_info "==========================================="

  if ! command -v task &> /dev/null; then
    log_info "Task not found"
    latest_task_version="$(get_latest_task_version)"
    install_task "${latest_task_version}"
  else
    current_version="$(task --version | awk '{print $2}')"
    latest_task_version="$(get_latest_task_version)"

    if version_ge "${current_version}" "${latest_task_version}"; then
      log_info "Task is up-to-date (version ${current_version})"
    else
      log_info "Task version ${current_version} is outdated. Updating to ${latest_task_version}"
      install_task "${latest_task_version}"
    fi
  fi
}

#######################################
# Main function
# Globals:
#   ORIGINAL_DIR
# Arguments:
#   Command line arguments
#######################################
main() {
  detect_system
  check_go_installation
  check_task_installation

  log_info "==========================================="
  log_info "Running 'go mod tidy' to ensure dependencies are up-to-date..."
  log_info "==========================================="

  if ! cd "${ORIGINAL_DIR}"; then
    log_error "Failed to return to original directory: ${ORIGINAL_DIR}"
  fi

  if [[ -f "go.mod" ]]; then
    if ! go mod tidy; then
      log_error "'go mod tidy' failed"
    fi
    log_info "'go mod tidy' completed successfully"
  else
    log_error "go.mod file not found in the directory: ${ORIGINAL_DIR}"
  fi

  log_info "==========================================="
  log_info "Bootstrap complete!"
  log_info "==========================================="
}

# Execute main with all script arguments
main "$@"
