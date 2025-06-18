use crate::errors::BoxedError;
use std::pin::Pin;

pub enum InstructionParam<TCtx> {
    String(String),
    Func(Box<dyn Fn(&TCtx) -> Result<String, BoxedError> + Send + Sync>),
    AsyncFunc(
        Box<
            dyn Fn(
                    &TCtx,
                )
                    -> Pin<Box<dyn futures::Future<Output = Result<String, BoxedError>> + Send>>
                + Send
                + Sync,
        >,
    ),
}

impl<TCtx> InstructionParam<TCtx> {
    pub async fn as_string(&self, context: &TCtx) -> Result<String, BoxedError> {
        match self {
            Self::String(s) => Ok(s.clone()),
            Self::Func(f) => f(context),
            Self::AsyncFunc(f) => f(context).await,
        }
    }
}

impl<TCtx> From<String> for InstructionParam<TCtx> {
    fn from(value: String) -> Self {
        Self::String(value)
    }
}

impl<TCtx> From<&str> for InstructionParam<TCtx> {
    fn from(value: &str) -> Self {
        Self::String(value.to_string())
    }
}

impl<TCtx, F> From<F> for InstructionParam<TCtx>
where
    F: Fn(&TCtx) -> Result<String, BoxedError> + Send + Sync + 'static,
    TCtx: Send + Sync + 'static,
{
    fn from(value: F) -> Self {
        Self::Func(Box::new(value))
    }
}

pub async fn get_prompt<TCtx>(
    instructions: &[InstructionParam<TCtx>],
    context: &TCtx,
) -> Result<String, BoxedError> {
    let results =
        futures::future::join_all(instructions.iter().map(|param| param.as_string(context))).await;
    Ok(results
        .into_iter()
        .collect::<Result<Vec<_>, _>>()?
        .join("\n"))
}
