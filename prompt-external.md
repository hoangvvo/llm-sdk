We are building a library to access LLM APIs that require convert from and between a set of unified APIs and interfaces.
To do that effectively, we can first generate OpenAPI schemas from the types above of respective providers (like OpenAI).

However, we don't need to translate all of them (not all schemas like ChatCompletionUpdateParams or properties of types like logprops) since we don't use all of them.
To know what needs to be translate, refer to what actually get used in the existing TypeScript library:

Some types are from other files which are not shown above, please add a TODO next to the type you are not sure and just leave it empty.
The outcome is a OpenAPI schema containing selective types from OPENAI (not our library!). Do not write description field unless it is for TODO. Again, if the properties or types are not found in the existing TypeScript library, don't include them in the OpenAPI schema.

You can find the OpenAI types and Existing TypeScript Library below in two separate code block.
