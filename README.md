# Meta

Metadata handling for CALPYR data platform

## Workflow -- General Design Paramters

This repo is designed to produce git hook commands that take care of metadata additions / subtractions that are run before or after certain git commands like commit and push. Draft workflow currently:

1. git add META directory to update metadata snapshot that will be sent to Calpyr ETL pipeline
   a. optionally, if you only have files and no metadata, add .meta file stubs to repo.

2. git commit. This will run the pre-commit forge hook that will translate .meta files into Document Reference entries and package it all up into a snapshot zip file. If you have additional metadata / document references along with your files that you are commiting the two processes shouldn't conflict with eachother. LFS hooks / lfs add is done in this as well for snapshot file.

3. git push. The post push hook command will start the sower job if the git-lfs upload doesn't error out. If it does, then this command isn't run. Essentially all it does is start a sower job with the DRS ids of the zip snapshot files that you want to upload. Initial server ETL design is to delete all data in the project and reload it, so if there are multiple snaphots, probably only want to upload the latest one.

4. A clone level hook will have to search the repo for all of the snapshots, pull the latest one and place it unzipped in the META directory. Additionally if you are pushing a new snapshot, but your META/ directory is old, ie: a snapshot has been pushed since you last cloned / or pulled the latest metadata into your repo there will need to be a system server side that can quickly detect this mismatch and return an error before the job is sent to the server. One idea might be to have the snapshot file always be called the same name so that if you attempt to push an old version of the file, git should save you with a merge error.

5. If the user is adding files locally, there should be a command that is run in the pre-commit hook for generating these ".meta" files so that they can be used for templating docrefs. This would probably be an add hook perhaps.

6. To setup these "forge" level git hooks a wrapper "init" command like "forge init" will probably have to be run which will wrap git-drs init
   and add in all of the special forge hooks into the .git directory so that it works as intended for the user.

## Example user workflow files only

```
git clone repo
forge init
git add files, --add hook is run to create .meta stubs
git commit -m "test" -- files packaged into snapshot file
git push origin main -- check
```

## Example user workflow metadata only

```
git clone repo
forge init
git add META/
git commit -m "test"
git push origin main
```

If git doesn't detect any changes with remote and your local git state you will not be able to do anything. This could be an issue
when metadata processes don't work as expected, but We should probably move to support local simulation so that users aren't trying to debug
pushes to a repo via the actual Calypr stack.
