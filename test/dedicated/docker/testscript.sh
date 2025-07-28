#!/usr/bin/env bash

# Make sure .env is present and readable
if [ -f .env ]; then
  source .env
fi

# Function to generate a superuser JWT (using the secret directly in Bash)
generate_superuser_jwt() {
  header=$(echo -n '{"alg":"HS256","typ":"JWT"}' | openssl base64 -e | tr -d '=' | tr '/+' '_-' | tr -d '\n')
  payload=$(echo -n '{"role":"postgres"}' | openssl base64 -e | tr -d '=' | tr '/+' '_-' | tr -d '\n')
  signature=$(echo -n "${header}.${payload}" | openssl dgst -sha256 -hmac "$JWT_SECRET" -binary | openssl base64 -e | tr -d '=' | tr '/+' '_-' | tr -d '\n')
  echo "${header}.${payload}.${signature}"
}

# Step 1: Get Superuser JWT
SUPERUSER_JWT=$(generate_superuser_jwt)
echo "Superuser JWT: $SUPERUSER_JWT"
echo "clearing cache"
curl -X POST http://localhost:3000/-/reload

# Testing a postgres user
echo "testing superuser test function"
TEST_USER_RESPONSE=$(curl -s -X POST "$API_URL/rpc/dummy_admin" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoicG9zdGdyZXMifQ.HnzAdDbTuozNKKGm3hNzzYfYdgQYid0Ii55vN43bIgY" \
  -H "Content-Type: application/json" \
  -d '{}')

echo "Test User Response: $TEST_USER_RESPONSE"



# Step 2: Create a New User
echo "Creating new user..."
CREATE_USER_RESPONSE=$(curl -s -i -X POST "$API_URL/rpc/create_user" \
  -H "Authorization: Bearer $SUPERUSER_JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "p_username": "testuser1",
    "p_password": "securepassword",
    "p_email": "testuser1@example.com"
  }')

echo "Create User Response: $CREATE_USER_RESPONSE"


# Step 3: Login as the New User
echo "Logging in as the new user..."
LOGIN_RESPONSE=$(curl -s -X POST "$API_URL/rpc/login_user" \
  -H "Content-Type: application/json" \
  -d '{
        "p_password": "securepassword",
        "p_username": "testuser1"
    }')

USER_JWT=$(echo $LOGIN_RESPONSE | sed -E 's/.*"([^"]+)".*/\1/')
echo "$API_URL :User JWT: $USER_JWT"


# Step 4: Create a New Task as the New User
echo "Creating a new task..."
CREATE_TASK_RESPONSE=$(curl -s -X POST "$API_URL/tasks" \
  -H "Authorization: Bearer $USER_JWT" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Other User:Complete PostgreSQL RLS setup",
    "completed": false
  }')

echo "Create Task Response: $CREATE_TASK_RESPONSE"

# Step 5: Retrieve Tasks for the User
echo "Fetching tasks for user..."
GET_TASKS_RESPONSE=$(curl -s -X GET "$API_URL/tasks" \
  -H "Authorization: Bearer $USER_JWT")

echo "User Tasks:"
echo "$GET_TASKS_RESPONSE"

echo "All steps completed successfully."