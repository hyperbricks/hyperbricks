
### Quickstart
Follow these steps to get started
#### 1. [Installation Instructions for HyperBricks](#installation-instructions-for-hyperbricks)
#### 2.	Initialize a new project:
```bash
hyperbricks init -m someproject
```

This creates a folder <someproject> in the modules directory in the root. Always run the hyperbricks cli commands the root (parent of modules directory), otherwise it will not find the module given by the -m parameter.

In the folder someproject you find this directory structure:
```
your_module_name/
  ├── hyperbricks
  ├────── hello_world.hyperbricks
  ├── rendered
  ├── resources
  ├── static
  ├── template
  └─ package.hyperbricks
```

#### 3.	Start the project:
```bash
hyperbricks start -m someproject 
```

HyperBricks will scan the hyperbricks root folder for files with the .hyperbricks extensions (not subfolders) and look for package.hyperbricks in the root of the module for global configurations.

for start options type:
```bash
hyperbricks start --help 
```

#### 3.	Access the project in the browser:
Open the web browser and navigate to http://localhost:8080 to view running hyperbricks.

### Installation Instructions for HyperBricks

Requirements:

- Go version 1.23.2 or higher

To install HyperBricks, use the following command:

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

This command downloads and installs the HyperBricks CLI tool

### Usage:
```
hyperbricks [command]
```
```
Available Commands:
-  completion  [Generate the autocompletion script for the specified shell]
-  help        [Help about any command]
-  init        [Create package.hyperbricks and required directories]
-  select      [Select a hyperbricks module]
-  start       [Start server]
-  static      [Render static content]
-  version     [Show version]

Flags:
  -h, --help   help for hyperbricks
```
Use "hyperbricks [command] --help" for more information about a command.

### Initializing a Project

To initialize a new HyperBricks project, use the `init` command:

```bash
hyperbricks init -m <name-of-hyperbricks-module>
```
without the -m and ```<name-of-hyperbricks-module>``` this will create a ```default``` folder.


This will create a `package.hyperbricks` configuration file and set up the required directories for the project.

---

### Starting a Module

Once the project is initialized, start the HyperBricks server using the `start` command:

```bash
hyperbricks start  -m <name-of-hyperbricks-module>
```

Use the --production flag when adding system and service manager in linux or on a mac
```bash
hyperbricks start  -m <name-of-hyperbricks-module> --production
```
This will launch the server, allowing you to manage and serve hypermedia content on the ip of the machine.

Or ```hyperbricks start``` for running the module named ```default```.

### Rendering static files to render directory

```bash
hyperbricks static  -m <name-of-hyperbricks-module>
```

### Additional Commands

HyperBricks provides other useful commands:



- **`completion`**: Generate shell autocompletion scripts for supported shells.
- **`help`**: Display help information for any command.

For detailed usage information about a specific command, run:

```bash
hyperbricks [command] --help
```