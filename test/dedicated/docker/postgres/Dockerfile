# Use the official PostgreSQL Alpine image (PostgREST requires PG >= 13)
FROM postgres:15-alpine

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    make \
    git \
    postgresql-dev

# Clone and install pgjwt extension
RUN git clone https://github.com/michelp/pgjwt.git /pgjwt && \
    cd /pgjwt && \
    make && \
    make install

# Clean up
RUN rm -rf /pgjwt
