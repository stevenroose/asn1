#!/bin/bash

# Add Travis badge:
cat > ./README.md << 'EOF'
[![Build Status](https://travis-ci.org/PromonLogicalis/asn1.svg?branch=master)](https://travis-ci.org/PromonLogicalis/asn1)
EOF

# Add Go doc
godocdown ./ >> ./README.md
