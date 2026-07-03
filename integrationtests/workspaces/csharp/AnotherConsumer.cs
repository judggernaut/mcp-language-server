namespace Workspace;

public static class AnotherConsumer
{
    // AnotherConsumerFunction is a second consumer of shared types and functions
    public static void AnotherConsumerFunction()
    {
        Console.WriteLine("Another message: " + Helper.HelperFunction());

        // Create another SharedClass instance
        var s = new SharedClass
        {
            Id = 2,
            Name = "another test",
            Value = 99.9,
            Constants = new List<string> { SharedConstants.SharedConstant, "extra" },
        };

        // Use the class methods
        var name = s.GetName();
        if (name != "")
        {
            Console.WriteLine("Got name: " + name);
        }

        // Implement the interface with a custom type through composition
        var custom = new CustomImplementor(s);

        // Custom type implements SharedInterface
        SharedInterface iface = custom;
        iface.Process();

        // Use shared type as a list
        var values = new List<SharedType> { SharedType.First, SharedType.Second, SharedType.Third };
        foreach (var v in values)
        {
            Console.WriteLine("Value: " + v);
        }
    }
}

// CustomImplementor implements SharedInterface by wrapping a SharedClass
public class CustomImplementor : SharedInterface
{
    private readonly SharedClass _inner;

    public CustomImplementor(SharedClass inner)
    {
        _inner = inner;
    }

    public void Process() => _inner.Process();

    public string GetName() => _inner.GetName();
}
