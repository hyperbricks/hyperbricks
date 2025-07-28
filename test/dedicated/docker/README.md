PostgreSQL, Row-Level Security (RLS), and PostgREST within a Docker-based multi-container environment.

PoC/Testing RLS, Basicly testing permissions on owner data, in this case owner_id of the tasks table

	•	User Authentication & Authorization
	•	Uses JWT (JSON Web Tokens) to identify and authorize users.
	•	Links JWT claims to PostgreSQL roles for fine-grained access control.
	•	Multi-User Access with RLS
	•	Ensures that users can only query or modify their own data.
	•	Defines policies in PostgreSQL that are enforced at the database level.
	•	Docker Multi-Container Environment
	•	Runs both PostgreSQL and PostgREST in separate containers.
	•	Might include additional services like pgAdmin for database management.

```
docker-compose up --build
```

Then run 
```
./testscript.sh
```

