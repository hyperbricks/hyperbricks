# Use the official PostgreSQL Alpine image (PostgREST requires PG >= 13)
FROM postgres:15-alpine

ARG PGJWT_COMMIT=f3d82fd30151e754e19ce5d6a06c71c20689ce3d

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    make \
    git \
    postgresql-dev

# Fetch the reviewed pgjwt revision and install the extension.
RUN git init /pgjwt && \
    git -C /pgjwt remote add origin https://github.com/michelp/pgjwt.git && \
    git -C /pgjwt fetch --depth 1 origin "${PGJWT_COMMIT}" && \
    git -C /pgjwt checkout --detach FETCH_HEAD && \
    cd /pgjwt && \
    make && \
    make install

# Clean up
RUN rm -rf /pgjwt
