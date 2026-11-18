#!/bin/sh

convert_version() { 
    printf "D-%02d.%03d.%03d" $(echo -n "$1" | sed 's/^v//' | tr '.' ' '); 
}

convert_version $1
