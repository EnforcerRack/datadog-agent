# FROM datadog/agent:latest

# # Copy Datadog Agent configuration files
# COPY datadog.yaml /etc/datadog.yaml

# # Set the environment variables needed to configure the Datadog Agent
# ENV DD_API_KEY="9357ee80-cb99-4678-8db2-997abaaa0a0e"
# ENV DD_APM_ENABLED=true
# ENV DD_LOGS_ENABLED=true

# COPY datadog.yaml /etc/datadog-agent/datadog.yaml

# # Expose the port that the Datadog Agent uses to receive data
# EXPOSE 8125/udp

# # Start the Datadog Agent
# # CMD ["agent"]

# CMD ["datadog-agent", "start"]

FROM centos:7

# Install Datadog Agent dependencies
RUN yum install -y wget

# Download the Datadog Agent installer
RUN DD_AGENT_MAJOR_VERSION=7 DD_API_KEY="9357ee80-cb99-4678-8db2-997abaaa0a0e" DD_SITE="datadoghq.com" \
    DD_INSTALL_ONLY=true \
    DD_LOGS_STDOUT=true \
    sh -c "$(wget https://raw.githubusercontent.com/DataDog/datadog-agent/master/cmd/agent/install_script.sh -O -)"

# Set up OpenShift user
RUN chgrp -R 0 /opt/datadog-agent && \
    chmod -R g=u /opt/datadog-agent && \
    useradd -u 1001 -r -g 0 -s /sbin/nologin -c "Default user" default

# Change to OpenShift user
USER 1001

# Start the Datadog Agent
CMD ["/opt/datadog-agent/bin/agent", "start"]
