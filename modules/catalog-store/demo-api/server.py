#!/usr/bin/env python3
"""Local demo API for the catalog-store HyperBricks module."""
import argparse
import json
from html.parser import HTMLParser
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import threading
from uuid import uuid4
from re import fullmatch
from urllib.parse import urlsplit, parse_qs, urlencode
from urllib.request import urlopen

DATA = json.loads((Path(__file__).parent/'catalog.json').read_text())
LOCK = threading.Lock()
IMAGE_LOCK = threading.Lock()
IMAGE_READY = False
SITE_URL = 'http://127.0.0.1:4320'
STATE = {'fail': False, 'requests': 0, 'failures': 0}
CARTS = {}
ITEMS = {item['id']: item for item in DATA}


class ImageManifestParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.current = None
        self.images = {}

    def handle_starttag(self, tag, attrs):
        attributes = dict(attrs)
        if tag == 'span':
            self.current = attributes.get('data-image-id')
        elif tag == 'img' and self.current:
            self.images[self.current] = attributes['src']

    def handle_endtag(self, tag):
        if tag == 'span':
            self.current = None


def ensure_images():
    """Ask HyperBricks to render IMAGE components and reuse their static URLs."""
    global IMAGE_READY
    if IMAGE_READY:
        return
    with IMAGE_LOCK:
        if IMAGE_READY:
            return
        with urlopen(SITE_URL + '/catalog-images', timeout=30) as response:
            markup = response.read().decode('utf-8')
        parser = ImageManifestParser()
        parser.feed(markup)
        if set(parser.images) != set(ITEMS):
            raise ValueError('HyperBricks did not render all 24 catalog images')
        for item in DATA:
            item['image'] = parser.images[item['id']]
        IMAGE_READY = True


def cart_view(session):
    quantities = CARTS.get(session, {})
    items = []
    subtotal = 0
    count = 0
    for item in DATA:
        quantity = quantities.get(item['id'], 0)
        if not quantity:
            continue
        price_cents = round(item['price'] * 100)
        subtotal += price_cents * quantity
        count += quantity
        items.append({
            **item, 'quantity': quantity,
            'increment': min(99, quantity + 1),
            'decrement': quantity - 1,
            'at_max': quantity >= 99,
            'price_label': f"${price_cents / 100:.2f}",
            'line_total_label': f"${price_cents * quantity / 100:.2f}",
        })
    return {
        'items': items, 'count': count, 'session': session,
        'subtotal_label': f"${subtotal / 100:.2f}",
        'total_label': f"${subtotal / 100:.2f}",
        'errors': {},
    }


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def send(self, code, value):
        body=json.dumps(value).encode()
        self.send_response(code)
        self.send_header('Content-Type','application/json; charset=utf-8')
        self.send_header('Cache-Control','no-store')
        self.send_header('Content-Length',str(len(body)))
        self.end_headers(); self.wfile.write(body)

    def require_images(self):
        try:
            ensure_images()
        except (OSError, UnicodeError, ValueError) as error:
            self.send(503, {'error': 'Start HyperBricks on the configured site URL to prepare catalog images', 'detail': str(error)})
            return False
        return True

    def session(self):
        value = self.headers.get('Authorization', '')
        token = value[7:] if value.startswith('Bearer ') else ''
        return token if token in CARTS else ''

    def do_GET(self):
        url=urlsplit(self.path)
        path=url.path
        if path=='/health': return self.send(200, {'ok':True})
        if path=='/__state':
            with LOCK: return self.send(200,dict(STATE))
        if path=='/cart':
            if not self.require_images(): return
            with LOCK:
                return self.send(200, cart_view(self.session()))
        if path=='/catalog':
            if not self.require_images(): return
            with LOCK:
                STATE['requests']+=1
                if STATE['fail']:
                    STATE['failures']+=1
                    return self.send(503,{'error':'Catalog temporarily unavailable'})
            query=parse_qs(url.query, keep_blank_values=True)
            if 'id' in query or query.get('view')==['detail']:
                ids=query.get('id',[])
                if not ids:
                    return self.send(404,{'error':'Item not found'})
                if len(ids)!=1 or not ids[0]:
                    return self.send(400,{'error':'Provide one non-empty item id'})
                item=next((item for item in DATA if item['id']==ids[0]),None)
                if item is None:
                    return self.send(404,{'error':'Item not found'})
                return self.send(200,item)
            if query.get('view')==['list']:
                search=query.get('search',[''])[0].strip()
                category=query.get('category',[''])[0]
                matches=[item for item in DATA if search.lower() in (item['name']+' '+item['description']).lower() and (not category or item['category']==category)]
                size=6
                pages=max(1,(len(matches)+size-1)//size)
                try: page=int(query.get('page',['0'])[0])
                except ValueError: page=0
                page=max(0,min(page,pages-1))
                def url_for(number):
                    return '/catalog-feed?'+urlencode({'search':search,'category':category,'page':number})
                return self.send(200, {
                    'items':matches[page*size:(page+1)*size],
                    'total':len(matches),'count_label':f"{len(matches)} {'item' if len(matches)==1 else 'items'}",
                    'page_label':f'Page {page+1} of {pages}',
                    'has_previous':page>0,'has_next':page<pages-1,
                    'previous_url':url_for(page-1),'next_url':url_for(page+1),
                    'filtered':bool(search or category)
                })
            return self.send(200,DATA)
        self.send(404,{'error':'Not found'})

    def do_POST(self):
        url = urlsplit(self.path)
        path = url.path
        if path in ('/__fail','/__reset'):
            with LOCK:
                STATE.update(fail=path=='/__fail',requests=0,failures=0)
            return self.send(200,{'ok':True})
        if path not in ('/cart/add','/cart/update','/checkout'):
            return self.send(404,{'error':'Not found'})
        if not self.require_images(): return
        query = parse_qs(url.query, keep_blank_values=True)
        with LOCK:
            session = self.session()
            if path in ('/cart/add','/cart/update'):
                ids = query.get('id', [])
                if len(ids) != 1 or ids[0] not in ITEMS:
                    return self.send(404, {'error':'Item not found'})
                item_id = ids[0]
                if path == '/cart/add':
                    if not session:
                        session = uuid4().hex
                        CARTS[session] = {}
                    CARTS[session][item_id] = min(99, CARTS[session].get(item_id, 0) + 1)
                    result = cart_view(session)
                    result['added'] = ITEMS[item_id]['name']
                    return self.send(200, result)
                quantities = query.get('quantity', [])
                if len(quantities) != 1 or not quantities[0].isdigit():
                    return self.send(400, {'error':'Invalid quantity'})
                quantity = int(quantities[0])
                if quantity > 99:
                    return self.send(400, {'error':'Invalid quantity'})
                if session:
                    if quantity:
                        CARTS[session][item_id] = quantity
                    else:
                        CARTS[session].pop(item_id, None)
                return self.send(200, cart_view(session))
            length = int(self.headers.get('Content-Length', '0'))
            if length > 4096:
                return self.send(413, {'error':'Form is too large'})
            try:
                fields = json.loads(self.rfile.read(length) or b'{}')
            except (ValueError, UnicodeDecodeError):
                fields = {}
            if not isinstance(fields, dict):
                fields = {}
            name = str(fields.get('name') or '').strip()
            email = str(fields.get('email') or '').strip()
            address = str(fields.get('address') or '').strip()
            errors = {}
            if len(name) < 2: errors['name'] = 'Enter your name.'
            if not fullmatch(r'[^@\s]+@[^@\s]+\.[^@\s]+', email): errors['email'] = 'Enter a valid email address.'
            if len(address) < 5: errors['address'] = 'Enter a delivery address.'
            if errors or not cart_view(session)['items']:
                result = cart_view(session)
                result['errors'] = errors
                result['error'] = 'Add an item before checking out.' if not result['items'] else ''
                result['entered'] = {'name': name, 'email': email, 'address': address}
                return self.send(200, result)
            CARTS[session] = {}
            return self.send(200, {
                'confirmed': True,
                'order_id': 'FORM-' + uuid4().hex[:8].upper(),
                'name': name, 'count': 0,
            })

if __name__=='__main__':
    parser=argparse.ArgumentParser(description='Run the catalog-store demo API on loopback')
    parser.add_argument('--port',type=int,default=4319)
    parser.add_argument('--site-url',default='http://127.0.0.1:4320',help='HyperBricks base URL for the IMAGE component route')
    args=parser.parse_args()
    SITE_URL = args.site_url.rstrip('/')
    print(f'Demo API: http://127.0.0.1:{args.port} (HyperBricks: {SITE_URL})', flush=True)
    ThreadingHTTPServer(('127.0.0.1',args.port),Handler).serve_forever()
