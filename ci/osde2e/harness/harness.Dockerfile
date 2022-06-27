FROM redhat/ubi8

WORKDIR /

# Update package database.
RUN yum makecache

# Install preliminary dependencies.
# Install oc.
RUN yum install -y wget tar git && \
    curl --remote-name https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp-dev-preview/latest/openshift-client-linux.tar.gz && \
    tar xzf openshift-client-linux.tar.gz && \
    mv oc /usr/local/bin/oc && \
    chmod +x /usr/local/bin/oc

# Install nodejs@15 (for cypress).
RUN curl -sL https://rpm.nodesource.com/setup_15.x | bash - && \
    yum install -y nodejs && \
    node --version && \
    npm --version && \
    npm i -g cypress yarn

# Enable CentOS 8 repository to fetch external Cypress dependencies.
RUN echo -e '[CentOS-8]\n\
name=CentOS-8\n\
baseurl=http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/\n\
enabled=1\n\
gpgcheck=1\n\
gpgkey=https://www.centos.org/keys/RPM-GPG-KEY-CentOS-Official'\
>> /etc/yum.repos.d/centos8.repo

# Install external dependencies (for cypress).
RUN rpm --import https://www.centos.org/keys/RPM-GPG-KEY-CentOS-Official && \
    yum install -y xorg-x11-server-Xvfb atk java-atk-wrapper at-spi2-atk gtk3 libXt mesa-libgbm libdrm

# Install ocm-cli.
RUN yum -y install yum-plugin-copr && \
    yum -y copr enable ocm/tools && \
    yum -y install ocm-cli && \
    ocm version

# Install jq.
RUN yum -y install jq

# TODO: Points to my branch, will replace after triggering a rehearsal on openshift/release and testing.
ARG ADDON_TEST_FILE="harness.bash"
ADD https://raw.githubusercontent.com/rexagod/mcg-osd-deployer/add-harness/ci/osde2e/harness/"${ADDON_TEST_FILE}" /usr/local/bin/
RUN chmod +x /usr/local/bin/${ADDON_TEST_FILE}

# Run the test suite script when the container is created.
ENV ENTRYPOINT_FILE="/usr/local/bin/${ADDON_TEST_FILE}"
ENTRYPOINT exec "$ENTRYPOINT_FILE"
