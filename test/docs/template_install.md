### Installation

To install HyperBricks, use the following command:

```bash
go install github.com/hyperbricks/hyperbricks/cmd/hyperbricks@latest
```

This command downloads and installs the HyperBricks CLI tool on your system.
---

### Initializing a Project

To initialize a new HyperBricks project, use the `init` command:

```bash
hyperbricks init -m <name-of-hyperbricks-module>
```

without the -m and ```<name-of-hyperbricks-module>``` this will create a ```default``` folder.


This will create a `package.hyperbricks` configuration file and set up the required directories for your project.

---

### Starting a Module

Once your project is initialized, start the HyperBricks server using the `start` command:

```bash
hyperbricks start  -m <name-of-hyperbricks-module>
```
This will launch the server, allowing you to manage and serve hypermedia content on the ip of your machine.

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