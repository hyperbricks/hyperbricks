#!/usr/bin/env python3
"""Copy the declared integration-fixture sources into a clean temporary project."""

import argparse
from pathlib import Path
import shutil


def source_files(module):
    """Read and validate the module's explicit source distribution list."""
    names = [line.strip() for line in (module / "SOURCE_FILES.txt").read_text().splitlines()
             if line.strip() and not line.lstrip().startswith("#")]
    if len(names) != len(set(names)):
        raise ValueError("SOURCE_FILES.txt contains duplicate paths")
    for name in names:
        relative = Path(name)
        if relative.is_absolute() or ".." in relative.parts:
            raise ValueError(f"Source path must stay inside the module: {name}")
        source = module / relative
        if source.is_symlink() or not source.is_file() or not source.resolve().is_relative_to(module.resolve()):
            raise ValueError(f"Source must be an existing regular file inside the module: {name}")
    return names


def stage(module, destination, static_profile=False):
    """Refuse to overwrite an existing project or copy undeclared generated data."""
    names = source_files(module)
    if static_profile and "package.static.hyperbricks.yaml" not in names:
        raise ValueError("The static profile must be declared in SOURCE_FILES.txt")
    if destination.exists():
        raise ValueError(f"Destination already exists; choose a new directory: {destination}")
    if destination.resolve().is_relative_to(module.resolve()):
        raise ValueError("Choose a destination outside the source module")
    target = destination / "modules" / "project-lifecycle-test"
    target.mkdir(parents=True)
    for name in names:
        output = target / name
        output.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(module / name, output)
    # The runtime validates configured directories even when no plugins or
    # rendered files are needed. These are empty workspace directories.
    (destination / "bin/plugins").mkdir(parents=True, exist_ok=True)
    (target / "rendered").mkdir(exist_ok=True)
    if static_profile:
        shutil.copy2(target / "package.static.hyperbricks.yaml", target / "package.hyperbricks.yaml")
    return target


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path, help="new project directory to create")
    parser.add_argument("--static-profile", action="store_true", help="select only the static export route")
    args = parser.parse_args()
    module = Path(__file__).resolve().parents[1]
    try:
        target = stage(module, args.destination.resolve(), static_profile=args.static_profile)
    except (OSError, ValueError) as error:
        parser.error(str(error))
    print(f"Sources staged at {target}")
    print("Run HyperBricks from the new project root with -m project-lifecycle-test.")


if __name__ == "__main__":
    main()
