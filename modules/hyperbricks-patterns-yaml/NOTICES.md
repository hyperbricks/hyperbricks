THIRD PARTY NOTICES
Module: hyperbricks-patterns-yaml

This notice covers this module's browser dependencies: HTMX and Tailwind CSS
used to build its frontend, and Unpoly loaded by its CDN-based example.
The source module contains imports and configuration, not vendored copies of
these libraries. Generated JavaScript/CSS is not included in the source archive.
Keep the applicable notices with frontend output when distributing that output.

These notices do not change the module's own license or cover separately
supplied HyperBricks runtime, compiled plugin binaries or the Tailwind CLI.

================================================================================
HTMX@4.0.0 — Zero-Clause BSD
Source: https://github.com/bigskysoftware/htmx/tree/v4.0.0

Imported by resources/js/htmx.js from the htmx.org npm package.
The repository package-lock.json resolves version 4.0.0. The esbuild component
bundles it into static/js/bundle.min.main.js.

Zero-Clause BSD
=============

Permission to use, copy, modify, and/or distribute this software for
any purpose with or without fee is hereby granted.

THE SOFTWARE IS PROVIDED “AS IS” AND THE AUTHOR DISCLAIMS ALL
WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES
OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE
FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY
DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN
AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT
OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

================================================================================
Tailwind CSS — MIT
Source: https://github.com/tailwindlabs/tailwindcss

Imported by resources/css/base.css. The repository package-lock.json resolves
the npm package to 4.2.4; the license text below is from that package.

hyperbricks/partials/tailwind.hyperbricks.yaml invokes an externally installed
tailwindcss executable to generate static/css/styles.css. The generated CSS
version follows that executable and may differ from the npm lockfile.
Check the generated CSS header and preserve the applicable upstream notices
when distributing a build made with a different tool version.

MIT License

Copyright (c) Tailwind Labs, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

================================================================================
Unpoly@3.14.3 — MIT (externally loaded)
Source: https://github.com/unpoly/unpoly

hyperbricks/85-unpoly-fragment-demo.hyperbricks.yaml loads these files remotely:
- https://cdn.jsdelivr.net/npm/unpoly@3.14.3/unpoly.css
- https://cdn.jsdelivr.net/npm/unpoly@3.14.3/unpoly.js

They are not copied into this source module. The following license is included
for reference and must stay with the library if it is bundled or self-hosted.

Copyright (c) 2014-2021 Henning Koch

MIT License

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

================================================================================
External font resource

hyperbricks/partials/site.hyperbricks.yaml loads Manrope through Google Fonts.
No Manrope font files are included in this source module. If font files are
later copied into a distribution, include their applicable upstream license.
