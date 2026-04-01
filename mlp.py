import torch
import torch.nn as nn

class SimpleMLP(nn.Module):
    """A simple multilayer perceptron with one hidden layer.

    - Input size: 10
    - Hidden layer size: 32 (ReLU activation)
    - Output size: 1
    """
    def __init__(self, input_dim: int = 10, hidden_dim: int = 32, output_dim: int = 1):
        super(SimpleMLP, self).__init__()
        self.net = nn.Sequential(
            nn.Linear(input_dim, hidden_dim),
            nn.ReLU(),
            nn.Linear(hidden_dim, output_dim)
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.net(x)

if __name__ == "__main__":
    # Create model instance
    model = SimpleMLP()
    # Dummy input: batch size 4, input dimension 10
    dummy_input = torch.randn(4, 10)
    # Forward pass
    output = model(dummy_input)
    print("Dummy input shape:", dummy_input.shape)
    print("Output shape:", output.shape)
    print("Output values:\n", output)
