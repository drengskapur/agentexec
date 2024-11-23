# AgentExec

AgentExec is a versatile, TUI-based application designed to manage and orchestrate resources across multiple services, including **Rancher**, **Harvester**, **Artifactory**, **Gitea**, and **Zarf**. It provides a streamlined interface for package management and API-based infrastructure automation, making complex multi-cloud workflows simple and intuitive.

## Features

- **Package Management**: Create, deploy, and manage packages of Docker images, Helm charts, and Git repositories using Zarf.
- **API Integration**: Seamless integration with external APIs, including Rancher, Harvester, Artifactory, and Gitea.
- **Custom Compression Control**: Allows the user to choose between speed-first, balanced, and size-first compression for packages.
- **User-Friendly TUI**: Powered by Bubble Tea, the TUI provides an accessible interface for complex cloud management workflows.

---

## Table of Contents

- [Architecture and Design Patterns](#architecture-and-design-patterns)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [Components](#components)
  - [1. `main.go` - TUI Entry Point](#1-maingo---tui-entry-point)
  - [2. `packager.go` - Package Creation](#2-packagergo---package-creation)
  - [3. `unpackager.go` - Package Deployment](#3-unpackagergo---package-deployment)
  - [4. `api.go` - API Integration](#4-apigo---api-integration)
  - [5. `Makefile` - Build and Dependency Management](#5-makefile---build-and-dependency-management)
- [Design Considerations](#design-considerations)
  - [TUI Design](#tui-design)
  - [Error Handling and Logging](#error-handling-and-logging)
  - [Dependency Management](#dependency-management)

---

## Architecture and Design Patterns

Omnivex follows a **modular, client-based architecture**, with each key function (e.g., packaging, API integration, TUI) encapsulated in its own component. This approach ensures clean separation of concerns and maintainability. 

- **Factory Pattern**: Used to initialize client connections (e.g., `APIClient`, `PackagerClient`, `UnpackagerClient`) based on user configurations and dependencies.
- **Command Pattern**: Commands for package management, deployment, and API requests are encapsulated in functions that execute specific commands (e.g., `ListGiteaRepos` in `api.go`).
- **Facade Pattern**: The TUI acts as a façade for interacting with complex workflows, allowing users to manage various services from a single interface without knowledge of underlying implementations.
- **Strategy Pattern**: Compression strategies (speed, balanced, size) are implemented as options in `packager.go`, giving users flexibility and simplicity.

---

## Installation

To get started with Omnivex, clone the repository and use the included `Makefile` to install dependencies, build, and run the application.

```bash
# Clone the repository
git clone https://github.com/drengskapur/agentexec.git
cd agentexec

# Install dependencies and build
make install-deps
make build

# Run the application
make run
```

---

## Usage

Once the application is running, the TUI will guide you through options to:
- View available packages
- Create and configure new packages
- Deploy packages to the desired environments
- Manage API interactions with Rancher, Harvester, Artifactory, and Gitea

### TUI Controls

- **`Enter`**: Select an option or view details.
- **`Esc`**: Go back to the previous screen.
- **`q`** or **`Ctrl+C`**: Quit the application.
- **`c`**: Cycle through compression levels (Speed, Balanced, Size) in the package manager.

### Compression Levels

- **Speed**: Emphasizes fast processing; uses `pigz` if available.
- **Balanced**: Balances speed and size; uses `zstd` with medium compression.
- **Size**: Emphasizes small file size; uses `zstd` with high compression.

---

## Configuration

Omnivex uses environment variables and a configuration file (optional) for specifying base URLs and tokens for API connections. This approach supports secure, environment-specific configurations without requiring hardcoded values.

Example `.env` file:

```env
RANCHER_URL="https://rancher.example.com"
RANCHER_TOKEN="your-rancher-token"
HARVESTER_URL="https://harvester.example.com"
HARVESTER_TOKEN="your-harvester-token"
ARTIFACTORY_URL="https://artifactory.example.com"
ARTIFACTORY_TOKEN="your-artifactory-token"
GITEA_URL="https://gitea.example.com"
GITEA_TOKEN="your-gitea-token"
```

---

## Components

### 1. `main.go` - TUI Entry Point

- **Function**: Initializes the TUI using Bubble Tea and orchestrates interactions between components.
- **User Experience**: Provides a simplified interface with a focus on accessibility. The TUI offers key commands, navigational feedback, and displays current compression levels.
- **Key Commands**: Handles navigation (`enter`, `esc`), quits (`q`), and toggles compression levels (`c`).

### 2. `packager.go` - Package Creation

- **Function**: Manages the creation of Zarf packages, including Docker images, Helm charts, and Git repositories.
- **Compression Strategy**: Provides configurable compression levels to balance speed and file size.
- **Dependencies**: Uses Zarf for packaging with fallback to `pigz` and `gzip` for compression. Includes checks to ensure dependencies are available.

### 3. `unpackager.go` - Package Deployment

- **Function**: Deploys Zarf packages and handles decompression as needed (e.g., `.zst`, `.gz`).
- **Error Handling**: Provides detailed feedback if deployment or decompression fails.
- **Decompression Strategy**: Detects file compression type and uses appropriate decompression tools (`unzstd`, `gunzip`) to prepare packages for deployment.

### 4. `api.go` - API Integration

- **Function**: Connects to external APIs like Rancher, Harvester, Artifactory, and Gitea.
- **Method Design**: Provides functions to list resources, create resources (e.g., virtual machines), and interact with repositories.
- **Error Handling**: Returns structured error messages and handles failed responses gracefully.
- **Reusable Patterns**: The `sendRequest` method consolidates HTTP requests for simplicity and reusability, supporting easy expansion for additional API methods.

### 5. `Makefile` - Build and Dependency Management

- **Function**: Provides targets to install dependencies, build the application, and run it.
- **Dynamic Updates**: Retrieves the latest Zarf version using GitHub’s API and automatically downloads the latest versions of required dependencies (e.g., Docker, Helm, Git).
- **Command Structure**: Includes clearly separated sections and comment blocks for easy navigation.

---

## Design Considerations

### TUI Design

The TUI leverages Bubble Tea’s model-update-view architecture for a responsive, easily navigable interface. The following design principles are emphasized:

- **Simplicity**: The TUI is minimalist and intuitive, guiding users through the management workflow without clutter.
- **Feedback**: Real-time feedback is provided for each action, such as selecting compression levels or viewing package details.
- **Accessibility**: Keybindings (`Enter`, `Esc`, `q`, `c`) are carefully chosen to facilitate easy navigation without requiring a mouse or extensive instructions.

### Error Handling and Logging

Omnivex includes structured error handling at each critical stage:

- **API Requests**: Returns clear error messages for connection issues or bad responses, including HTTP status and response body.
- **Package Management**: Detects and logs issues during package creation, deployment, or decompression.
- **Logging**: Omnivex provides essential log outputs directly in the terminal, which can be easily redirected or saved for deeper debugging if needed.

### Dependency Management

Dependencies are managed through the `Makefile`, ensuring that each required tool (e.g., Docker, Helm, Git, pigz) is available in the environment. By dynamically retrieving the latest versions of Zarf and other tools, Omnivex ensures compatibility with recent API updates and leverages improvements in performance and security.

---

## Future Enhancements

- **Multi-user TUI**: Add multi-user support for concurrent access.
- **Additional API Integrations**: Expand to other cloud-native tools like Kubernetes.
- **Export Logs**: Allow users to save log files of packaging and deployment actions.
- **Customization**: Provide options to customize color schemes and key bindings within the TUI.

---

## Conclusion

Omnivex is designed to provide a seamless, user-friendly interface for managing complex, multi-cloud workflows. By using modern design patterns, providing flexible compression options, and focusing on simplicity and clarity in the TUI, Omnivex empowers users to easily manage and deploy resources across multiple services.
