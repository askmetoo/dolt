#!/bin/sh

pwd
ls -a
dolt config --global --add metrics.disabled true
dolt config --global --add metrics.host localhost
dolt config --global --add user.name mysql-test-runner
dolt config --global --add user.email mysql-test-runner@liquidata.co

bats -v

results=$(bats /mysql-client-tests/mysql-client-tests.bats)
echo "::set-output name=results::$results"