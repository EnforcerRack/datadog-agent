# FROM registry.access.redhat.com/ubi8/ubi:latest

# USER root

# # Install required packages
# RUN rpm -y update && \
#     rpm -y install curl gnupg && \
#     rpm clean all

# # Install the Datadog Agent
# RUN DD_API_KEY=9357ee80-cb99-4678-8db2-997abaaa0a0e bash -c "$(curl -L https://s3.amazonaws.com/dd-agent/scripts/install_script_agent7.sh)"

# # Copy the Datadog Agent configuration file
# COPY datadog.yaml /etc/datadog-agent/datadog.yaml

# # Change ownership of the configuration file to the Datadog Agent user
# RUN chown -R dd-agent:dd-agent /etc/datadog-agent

# # Set the user to run the Datadog Agent
# USER dd-agent

# # Start the Datadog Agent
# CMD ["/opt/datadog-agent/bin/agent", "start"]

# FROM ubuntu:latest

# RUN apt-get update && apt-get install -y apt-transport-https gnupg curl
# RUN sh -c "echo 'deb https://apt.datadoghq.com/ stable main' > /etc/apt/sources.list.d/datadog.list"
# RUN curl -sL 'https://keys.datadoghq.com/DATADOG_RPM_KEY.public' | apt-key add -
# RUN apt-get update && apt-get install -y datadog-agent

# COPY datadog.yaml /etc/datadog-agent/datadog.yaml

# CMD ["datadog-agent", "start"]

FROM datadog/agent:latest

# Install dependencies
RUN rpm -y update && \
    rpm -y install wget && \
    wget -q https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm && \
    rpm -ivh epel-release-latest-7.noarch.rpm && \
    rpm -y install python-pip && \
    pip install requests && \
    pip install datadog && \
    rm -f epel-release-latest-7.noarch.rpm && \
    rpm clean all

# Copy Datadog Agent configuration files
COPY datadog.yaml /etc/datadog.yaml
COPY conf.d/* /etc/datadog-agent/conf.d/

# Set the environment variables needed to configure the Datadog Agent
ENV DD_API_KEY="9357ee80-cb99-4678-8db2-997abaaa0a0e"
ENV DD_APM_ENABLED=true
ENV DD_LOGS_ENABLED=true

# COPY datadog.yaml /etc/datadog-agent/datadog.yaml

# Expose the port that the Datadog Agent uses to receive data
EXPOSE 8125/udp

# Start the Datadog Agent
# CMD ["agent"]

CMD ["datadog-agent-git", "start"]