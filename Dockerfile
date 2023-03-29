FROM registry.access.redhat.com/ubi8/ubi:latest

USER root

# Install required packages
RUN yum -y update && \
    yum -y install curl gnupg && \
    yum clean all

# Copy the Datadog Agent configuration file
COPY datadog.yaml /etc/datadog-agent/datadog.yaml

# Install the Datadog Agent
RUN DD_API_KEY=9357ee80-cb99-4678-8db2-997abaaa0a0e bash -c "$(curl -L https://raw.githubusercontent.com/DataDog/datadog-agent/master/cmd/agent/install_script.sh)"

# Change ownership of the configuration file to the Datadog Agent user
RUN chown -R dd-agent:dd-agent /etc/datadog-agent

# Set the user to run the Datadog Agent
USER dd-agent

# Start the Datadog Agent
CMD ["/opt/datadog-agent/bin/agent", "start"]
