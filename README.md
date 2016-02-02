# Sentinel Manager


Sure, you can redis-cli into the Sentinels directly and do stuff
manually, but that can be error prone. Sentinel-manager is a tool to
wrap up various tasks and techniques for Sentinel actions and commands
into a smooth, easy to use interface. Where possible you can enact these
actions across multiple Sentinels.


# Add Pod

This command will easily add a pod to the given sentinel

# Set Pod Options in Sentinel

Each pod has a list of configuration directives such as password, how
many parallel syncs are allowed, failover timeouts, etc.. This
subcommand will easily handle that for you - even across all known
sentinels if you like!


# Reset Pod

Sometimes you need to reset a pod, and usually you'll need to do it on
all known sentinels. Either way, this sub-command has you covered.

# Remove Pod

Sometimes you need to remove a pod, and usually you'll need to do it on
all known sentinels for that pod. Either way, this sub-command has you covered.

