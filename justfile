# Start Postgres and the API.
up:
    docker compose up -d --build

reset:
    docker compose down -v
    docker compose up -d --build

# Stop everything, keeping the database.
down:
    docker compose down
