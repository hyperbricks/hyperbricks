#!/bin/bash

# Define the URL to test (Change this to your API)
TEST_URL="https://example.com"

# First request (save cookies to cookies1.txt)
echo "Making first request to $TEST_URL..."
curl -i -c cookies1.txt -b cookies1.txt "$TEST_URL" > response1.txt

# Second request (save cookies to cookies2.txt)
echo "Making second request to $TEST_URL..."
curl -i -c cookies2.txt -b cookies2.txt "$TEST_URL" > response2.txt

# Compare cookie files
echo "Comparing cookies..."
diff cookies1.txt cookies2.txt > /dev/null

if [ $? -eq 0 ]; then
    echo "❌ Test Failed: Cookies are shared between requests!"
else
    echo "✅ Test Passed: Cookies are isolated per request."
fi

# Cleanup temporary files
rm cookies1.txt cookies2.txt response1.txt response2.txt