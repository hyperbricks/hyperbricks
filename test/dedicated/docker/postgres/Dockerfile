# Use the official PostgreSQL Alpine image
FROM postgres:12.3-alpine

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