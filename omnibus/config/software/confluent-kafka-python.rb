name "confluent-kafka-python"
default_version "2.0.2"

dependency "pip3"
dependency "librdkafka"

source :url => "https://github.com/confluentinc/confluent-kafka-python/archive/refs/tags/v#{version}.tar.gz",
       :sha256 => "137c82d23e931e03a2803569d51bb025a7b9a364818b08d1664800abaaa5cc63",
       :extract => :seven_zip

relative_path "confluent-kafka-python-#{version}"

build do
  license "Apache-2.0"
  license_file "./LICENSE.txt"

  # https://github.com/confluentinc/confluent-kafka-python/blob/master/INSTALL.md#install-from-source

  if windows?
    pip = "#{windows_safe_path(python_3_embedded)}\\Scripts\\pip.exe"
  else
    pip = "#{install_dir}/embedded/bin/pip3"
  end

  command "#{pip} install -std=gnu99 --no-binary confluent-kafka ."
end
