# FROM registry.access.redhat.com/ubi8/ubi:latest

# USER root

# # Install required packages
# RUN yum -y update && \
#     yum -y install curl gnupg && \
#     yum clean all

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

# Use the official Datadog Agent image
FROM datadog/agent:latest

# Set the OpenShift user ID as the agent user ID
USER root

# Install necessary packages
RUN yum install -y nc

# Expose the port that the agent will use to communicate with Datadog
EXPOSE 8125/udp

# Copy the Datadog configuration file
COPY datadog.yaml /etc/datadog-agent/datadog.yaml

# Set the agent command
CMD ["agent"]

# Set the OpenShift user ID as the agent user ID
USER 1001