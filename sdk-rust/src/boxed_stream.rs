use futures::{stream::BoxStream, Stream};
use std::{
    pin::Pin,
    task::{Context, Poll},
};

/// Generic wrapper to expose a boxed stream with a consistent interface.
pub struct BoxedStream<'a, T>(BoxStream<'a, T>);

impl<'a, T> BoxedStream<'a, T> {
    pub fn from_stream<S>(stream: S) -> Self
    where
        S: Stream<Item = T> + Send + 'a,
    {
        Self(Box::pin(stream))
    }
}

impl<'a, T> Stream for BoxedStream<'a, T> {
    type Item = T;

    fn poll_next(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Option<Self::Item>> {
        self.0.as_mut().poll_next(cx)
    }
}
