# groove

## Development

Groove is a separate golang executable but it has Python and Node APIs that you often want to co-develop. When developing locally you can make use of the `build.sh` script that will manually build the go project and place it into the necessary bin paths for Python and Node to start using it.

If you'd like to continuously compile changes to the golang proxy in the background:

```
cd groove-python && poetry run watchmedo shell-command --command="cd ../ && bash ./build.sh" ../proxy
```
