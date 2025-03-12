Here are some clear and usable HyperBricks examples:

---

### **Fragment Example with Response Headers**
```properties
fragment = <FRAGMENT>
fragment.route = fragment_response
fragment {
    content = <TREE>
    content {
        10 = <HTML>
        10.value = <p>This is a fragment with response headers.</p>
    }
    response {
        hx_trigger = customEvent
        hx_target = #response-container
    }
}
```
This fragment is triggered by `customEvent` and targets `#response-container` for content updates.

---

### **Hypermedia with Template**
```properties
hypermedia = <HYPERMEDIA>
hypermedia.route = template_page
hypermedia.title = Template Example
hypermedia.10 = <TEMPLATE>
hypermedia.10 {
    inline = <<[
        <h1>{{title}}</h1>
        <p>{{content}}</p>
    ]>>
    values {
        title = Welcome!
        content = This is a template-driven hypermedia page.
    }
}
```
This Hypermedia uses a template to structure its content dynamically.

---

### **Hypermedia with Multiple Ordered Items**
```properties
hypermedia = <HYPERMEDIA>
hypermedia.route = ordered_content
hypermedia.title = Ordered Items
hypermedia.10 = <HTML>
hypermedia.10.value = <p>Item 10</p>

hypermedia.20 = <HTML>
hypermedia.20.value = <p>Item 20</p>

hypermedia.30 = <HTML>
hypermedia.30.value = <p>Item 30</p>
```
Content is ordered numerically and renders in that sequence.

---

### **API Render Example**
```properties
api_render = <API_RENDER>
api_render.route = api_example
api_render.url = https://api.example.com/data
api_render.method = GET
api_render.inline = <<[
    <h1>{{.title}}</h1>
    <p>{{.description}}</p>
]>>
```
Fetches data from an API and renders it using a template.

---

### **API Fragment Render Example**
```properties
api_fragment = <API_FRAGMENT_RENDER>
api_fragment.route = api_fragment_example
api_fragment.url = https://api.example.com/fragment
api_fragment.method = POST
api_fragment.inline = <<[
    <div>{{.content}}</div>
]>>
api_fragment.response {
    hx_target = #fragment-container
    hx_trigger = newData
}
```
This API fragment fetches data and dynamically updates `#fragment-container`.

---

### **Image Example**
```properties
hypermedia = <HYPERMEDIA>
hypermedia.route = image_example
hypermedia.title = Image Display
hypermedia.10 = <IMAGE>
hypermedia.10 {
    src = https://picsum.photos/400
    width = 400
    height = 300
    alt = Random Image
}
```
Loads a placeholder image dynamically.



```properties
# Define the main Hypermedia document
hypermedia = <HYPERMEDIA>

# The route determines the URL path for this Hypermedia document (e.g., "/index")
hypermedia.route = index

# Title of the page (used in the document title and as a variable in the template)
hypermedia.title = Structured Page

# Defines the <body> tag attributes, such as a background color and padding
# The "|" character separates the opening and closing tag
hypermedia.bodytag = <body class="bg-gray-100 p-4">|</body>

# The <HEAD> section of the document, automatically available in <HYPERMEDIA>
# This can contain meta tags, stylesheets, and scripts
hypermedia.head {
    # Assigning priority 100 to ensure this block loads properly
    100 = <HTML>
    100.value = <<[
        <!-- Meta tags for character encoding and viewport settings -->
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">

        <!-- Internal CSS for basic styling -->
        <style>
            body { font-family: Arial, sans-serif; margin: 20px; }
            header, footer { background: #333; color: white; padding: 10px; text-align: center; }
            main { padding: 20px; }
        </style>

        <!-- External CSS: TailwindCSS for modern utility-based styling -->
        <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css">

        <!-- External JavaScript: HTMX for handling dynamic updates without full-page reloads -->
        <script src="https://unpkg.com/htmx.org@2.0.4"></script>
    ]>>
}

# The template is already predefined in <HYPERMEDIA>
# It determines the full structure of the HTML document and dynamically injects content
hypermedia.template {
    inline = <<[
        <!DOCTYPE html>
        <html lang="en">
        
        <!-- The head section is injected dynamically by hyperbricks using {{.head}}, 
             which pulls from hypermedia.head (not values) -->
        {{.head}}

        <body>
            <header>
                <!-- Injects the title dynamically from hypermedia.template.values.title -->
                <h1>{{.title}}</h1>
            </header>
            <main>
                <!-- Injects dynamic content from hypermedia.values.content -->
                <p>{{.content}}</p>
            </main>
            <footer>
                <p>&copy; 2025 My Website</p>
            </footer>
        </body>
        </html>
    ]>>

    # Predefined values injected into the template
    values {
        # This is referenced in the template with {{.title}}
        title = Structured Page
          
        # Used in {{.content}}
        content = This is a Hypermedia document with a full HTML structure.  
    }
}
```
This result in this html
```html
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
        }

        header,
        footer {
            background: #333;
            color: white;
            padding: 10px;
            text-align: center;
        }

        main {
            padding: 20px;
        }
    </style>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css">
    <script src="https://unpkg.com/htmx.org@2.0.4"></script>
    <meta name="generator" content="hyperbricks cms">
    <title>Structured Page</title>
</head>

<body>
    <header>
        <h1>Structured Page</h1>
    </header>
    <main>
        <p>This is a Hypermedia document with a full HTML structure.</p>
    </main>
    <footer>
        <p>&copy; 2025 My Website</p>
    </footer>
</body>

</html>
```