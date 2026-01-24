Plugin that can return HTML, a renderable map, or a TypeRequest.

**Example config:**
```
render_map = <PLUGIN>
render_map.plugin = RenderMapPlugin__test-004-plugins@1.0.0
render_map.data.mode = map
render_map.data.message = Hello from map

render_request = <PLUGIN>
render_request.plugin = RenderMapPlugin__test-004-plugins@1.0.0
render_request.data.mode = request
render_request.data.message = Hello from TypeRequest
```
