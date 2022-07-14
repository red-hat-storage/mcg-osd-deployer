FROM redhat/ubi8

WORKDIR /

# Configure k8s repository to install kubectl packages.
# COPY k8s.repo /etc/yum.repos.d/k8s.repo

# Python package installation.
RUN INSTALL_PKGS="python38 python38-devel python38-setuptools python38-pip \
      libffi-devel libcurl-devel openssl-devel libxslt-devel libxml2-devel libtool-ltdl enchant glibc-langpack-en redhat-rpm-config \
      git gcc" && \
    yum -y module enable python38:3.8 && \
    yum -y --setopt=tsflags=nodocs install $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum -y clean all --enablerepo='*' && \
    rm -rf /var/cache/yum && \
    curl -LO "https://dl.k8s.io/release/$(curl -L -s \
    https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    mv kubectl /bin/kubectl

# Install nodejs@15 (for cypress)
RUN curl -sL https://rpm.nodesource.com/setup_15.x | bash - && \
    yum install -y nodejs && \
    node --version && \
    npm --version && \
    npm i -g cypress yarn

# Points to my branch, will replace after triggering a rehearsal on openshift/release and testing.
ARG ADDON_TEST_FILE="harness.bash"
ADD https://raw.githubusercontent.com/rexagod/mcg-osd-deployer/add-harness/hack/harness/harness.bash /
COPY ${ADDON_TEST_FILE} /usr/local/bin/
RUN chmod +x /usr/local/bin/${ADDON_TEST_FILE}

# Run the test suite script when the container is created.
ENV ENTRYPOINT_FILE="/usr/local/bin/${ADDON_TEST_FILE}"
ENTRYPOINT exec "$ENTRYPOINT_FILE"
